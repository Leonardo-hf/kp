package main

import (
	"context"
	"io"
	"log"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"kp/msg"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/jsonx"
	"google.golang.org/protobuf/proto"
)

type GithubKPConfig struct {
	Datasets  string
	Addr      string
	Topic     string
	CInterval int64
	PInterval int64
	NotSleep  bool
}

// LoadFromEnv 从环境变量中读入配置，方便在 K8S 文件中调整配置
func LoadFromEnv() *GithubKPConfig {
	c := &GithubKPConfig{}
	t := reflect.TypeOf(c).Elem()
	v := reflect.ValueOf(c).Elem()
	for i := 0; i < t.NumField(); i++ {
		e := os.Getenv(t.Field(i).Name)
		switch t.Field(i).Type.Kind() {
		case reflect.String:
			v.Field(i).SetString(e)
		case reflect.Int64, reflect.Int:
			ei, _ := strconv.ParseInt(e, 10, 64)
			v.Field(i).SetInt(ei)
		case reflect.Bool:
			eb, _ := strconv.ParseBool(e)
			v.Field(i).SetBool(eb)
		}
	}
	return c
}

// GetGithubInfos 从数据集中获取所有 GithubInfo 并按更新时间排序
func GetGithubInfos(path string) GithubInfos {
	file, err := os.Open(path)
	if err != nil {
		log.Panicf("failed to open text file: %s, err: %v", path, err)
	}
	defer func() { _ = file.Close() }()
	json, _ := io.ReadAll(file)
	res := make(map[string]*OriginGithubInfo)
	_ = jsonx.UnmarshalFromString(string(json), &res)
	origins := make(GithubInfos, 0)
	for repo, info := range res {
		info.Repo = repo
		content := &OriginContent{}
		_ = jsonx.UnmarshalFromString(info.RawContent, content)
		info.Content = content
		origins = append(origins, info)
	}
	sort.Sort(origins)
	return origins
}

// Origin2Msg 将数据集中的原始 JSON 格式转化为 kafka 传递的 proto 编码格式
func Origin2Msg(o *OriginGithubInfo) []byte {
	deps := make([]*msg.Dependency, 0)
	if o.Content.Dependencies != nil {
		for k, v := range o.Content.Dependencies {
			v = strings.ReplaceAll(v, "~", "")
			v = strings.ReplaceAll(v, ">", "")
			v = strings.ReplaceAll(v, "=", "")
			v = strings.ReplaceAll(v, "^", "")
			v = strings.ReplaceAll(v, "x", "0")
			v = strings.ReplaceAll(v, " ", "")
			deps = append(deps, &msg.Dependency{
				Package: k,
				Version: v,
			})
		}
	}
	m, _ := proto.Marshal(&msg.GithubKPMsg{
		Repo:         o.Repo,
		Star:         o.Star,
		Fork:         o.Fork,
		Watch:        o.Watch,
		Dependencies: deps,
		Timestamp:    o.Timestamp.UnixMilli(),
		Description:  o.Content.Description,
		License:      o.Content.License,
		Homepage:     o.Content.Homepage,
		Keywords:     o.Content.Keywords,
		Author:       o.Content.Author,
	})
	return m
}

type OriginGithubInfo struct {
	Repo       string
	Star       int64  `json:"subscribers_count"`
	Fork       int64  `json:"forks_count"`
	Watch      int64  `json:"watchers"`
	RawContent string `json:"content"`
	Content    *OriginContent
	Timestamp  time.Time `json:"updated_at"`
}

type GithubInfos []*OriginGithubInfo

func (a GithubInfos) Len() int {
	return len(a)
}

func (a GithubInfos) Less(i int, j int) bool {
	return a[i].Timestamp.Before(a[j].Timestamp)
}

func (a GithubInfos) Swap(i int, j int) {
	a[i], a[j] = a[j], a[i]
}

type OriginContent struct {
	Dependencies map[string]string `json:"dependencies"`
	Description  string            `json:"description"`
	License      string            `json:"license"`
	Author       string            `json:"author"`
	Keywords     []string          `json:"keywords"`
	Homepage     string            `json:"homepage"`
}

func main() {
	c := LoadFromEnv()
	conn := &kafka.Writer{
		Addr:                   kafka.TCP(c.Addr),
		Topic:                  c.Topic,
		AllowAutoTopicCreation: true,
	}
	defer func() { _ = conn.Close() }()
	origins := GetGithubInfos(c.Datasets)
	if !c.NotSleep {
		// 等待到整点开始执行
		t := time.Now()
		sleep := time.NewTimer(time.Duration(59-t.Minute())*time.Minute + time.Duration(60-t.Second())*time.Second)
		<-sleep.C
	}
	// 每 PInternal 时间生产每 CInternal 时间内更新的 Github 项目
	ticker := time.NewTicker(time.Duration(c.PInterval) * time.Second)
	if len(origins) == 0 {
		return
	}
	idx := 0
	initTime := origins[0].Timestamp
	for range ticker.C {
		next := initTime.Add(time.Duration(c.CInterval) * time.Second)
		initTime = next
		for {
			if idx >= len(origins) || origins[idx].Timestamp.After(next) {
				break
			}
			idx++
			m := Origin2Msg(origins[idx])
			err := conn.WriteMessages(
				context.Background(),
				kafka.Message{Value: m},
			)
			if err != nil && err != kafka.UnknownTopicOrPartition {
				log.Fatalf("failed to write msg, err: %v", err)
			}
		}
		if idx >= len(origins) {
			ticker.Stop()
			break
		}
	}
}
