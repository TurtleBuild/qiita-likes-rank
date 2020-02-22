package main

import (
	"context"
	"gopkg.in/ini.v1"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
)

// Event is my event
type Event struct {
	Tag string `json:"tag"`
}

// Data is my response
type Data struct {
	LikesCount int
	Title      string
	URL        string
	Tags       []struct {
		Name string
	}
	Tag string
	LikesRank int
}

// Config is struct of config.ini
type Config struct{
	TableName string
	TruncRank int
}

// Cnf is stored Config values
var Cnf Config

// 初期化（Configのロード）
func init(){
	c, _ := ini.Load("config.ini")
	Cnf = Config{
		TableName: c.Section("DB").Key("tableName").String(),
		TruncRank: c.Section("spec").Key("truncRank").MustInt(),
	}
}

// ハンドラ
func handler(ctx context.Context, event Event) ([]Data, error) {

	// セッション
	sess, err := session.NewSession()
	if err != nil {
		panic(err)
	}
	// DB定義
	db := dynamo.New(sess)
	table := db.Table(Cnf.TableName)

	var datas []Data
	for rank:=1; rank<=Cnf.TruncRank; rank++ {
		err := table.Get("LikesRank", rank).Range("Tag", dynamo.Equal, event.Tag).All(&datas)
		if err != nil {
			break
		}
	}
	// データが取れない場合は空のスライスを返す
	if datas == nil {
		datas = []Data{}
	}

	return datas, nil
}

func main() {
	// ラムダ実行
	lambda.Start(handler)
}
