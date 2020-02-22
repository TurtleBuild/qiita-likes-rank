package main

import (
	"encoding/json"
	"gopkg.in/ini.v1"
	"fmt"
	"sort"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
)

// Data is used to receive response data of QiitaAPI and to register data to dynamoDB
type Data struct {
	LikesCount int       `json:"likes_count"`
	Title      string    `json:"title"`
	URL        string    `json:"url"`
	Tags       []struct {
		Name string `json:"name"`
	} `json:"tags"`
	Tag string
	LikesRank int
	RegistDate time.Time
}

// Config is struct of config.ini
type Config struct{
	QiitaToken string
	TableName string
	MaxPage int
	PerPage int
	Stocks int
	Tags []string
	TargetDays int
	TruncRank int
}

// Cnf is stored Config values
var Cnf Config

// 初期化（Configのロード）
func init(){
	c, _ := ini.Load("config.ini")
	Cnf = Config{
		QiitaToken: c.Section("auth").Key("qiitaToken").String(),
		TableName: c.Section("DB").Key("tableName").String(),
		MaxPage: c.Section("param").Key("maxPage").MustInt(),
		PerPage: c.Section("param").Key("perPage").MustInt(),
		Stocks: c.Section("param").Key("stocks").MustInt(),
		Tags: c.Section("param").Key("tags").Strings(","),
		TargetDays: c.Section("spec").Key("targetDays").MustInt(),
		TruncRank: c.Section("spec").Key("truncRank").MustInt(),
	}
}

// Qiita記事データをランキング形式で登録する
func registQiitaData(table dynamo.Table, datas []Data, tag string) {
	// いいね数で降順にソートする
	sort.Slice(datas, func(i, j int) bool { return datas[i].LikesCount > datas[j].LikesCount })

	likesRank := 1
	for _, data := range datas {
		data.LikesRank = likesRank
		data.Tag = tag
		data.RegistDate = time.Now()
		// DB登録
		if err := table.Put(data).Run(); err != nil {
			log.Print(err.Error())
		}
		likesRank++
		// 足切りまで達した場合
		if likesRank > Cnf.TruncRank {
			break
		}
	}
}

// QiitaAPIからレスポンスを取得
func requestQiitaAPI(accessToken string, endpointURL *url.URL) []Data {

	var response = &http.Response{}
	// Qiitaのアクセストークンがない場合はAuthorizationを付与しない
	if len(accessToken) > 0 {
		response, _ = http.DefaultClient.Do(&http.Request{
			URL:    endpointURL,
			Method: "GET",
			Header: http.Header{
				"Content-Type":  {"application/json"},
				"Authorization": {"Bearer " + accessToken},
			},
		})
	} else {
		response, _ = http.DefaultClient.Do(&http.Request{
			URL:    endpointURL,
			Method: "GET",
			Header: http.Header{
				"Content-Type": {"application/json"},
			},
		})
	}
	defer response.Body.Close()

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Print("Unable to read response:", err)
		return nil
	}

	var datas []Data

	if err := json.Unmarshal(responseBody, &datas); err != nil {
		log.Print("JSON Unmarshal error:", err)
		return nil
	}
	return datas
}

// エンドポイントURLの作成
func makeEndpointURL(pageNum int, perPage int, targetDate string, stocks int, tag string) *url.URL {
	// ベースのURL
	baseURL := "https://qiita.com/api/v2/"
	// アクション
	action := "items"
	// 基本パラメータ
	baseParam := "?page=" + strconv.Itoa(pageNum) + "&per_page=" + strconv.Itoa(perPage)
	// クエリパラメータ
	queryParam := "&query=created:>=" + targetDate + "+stocks:>=" + strconv.Itoa(stocks)
	// タグがあればクエリとして付与
	if tag != "overall" {
		queryParam += "+tag:" + tag
	}
	// エンドポイントURL
	endpointURL, err := url.Parse(baseURL + action + baseParam + queryParam)
	if err != nil {
		log.Print("URL:", endpointURL, " Parse error:", err)
		return nil
	}
	return endpointURL
}

// Qiita記事データを蓄積する
func addQiitaData(targetDate string, stocks int, tag string) []Data {
	var datas []Data

	for pageNum:=1; pageNum <= Cnf.MaxPage; pageNum++ {
		endpointURL := makeEndpointURL(pageNum, Cnf.PerPage, targetDate, stocks, tag)
		// 記事データの蓄積
		if endpointURL != nil {
			tmpDatas := requestQiitaAPI(Cnf.QiitaToken, endpointURL)
			if len(tmpDatas) == 0 {
				log.Print("全記事取得完了しました")
				break
			} else if tmpDatas == nil {
				break
			}
			datas = append(datas, tmpDatas...)
		}
	}
	return datas
}

// 年月日の数値を文字列に変換
func dateNumToString(targetDate time.Time) string {
	year, month, day := targetDate.Date()
	return fmt.Sprintf("%d-%d-%d", year, int(month), day)
}

// Qiita記事情報の削除処理
func deleteQiitaArticle(table dynamo.Table, tag string) {
	for rank:=1; rank<=Cnf.TruncRank; rank++ {
		err := table.Delete("LikesRank", rank).Range("Tag", tag).Run()
		if err !=nil {
			log.Print("LikesRank:", rank, " Tag:", tag, " の削除に失敗しました, error:", err)
			break
		}
	}
}

// ハンドラ
func handler() {
	// セッション
	sess, err := session.NewSession()
	if err != nil {
		log.Print("セッション確立に失敗しました")
		panic(err)
	}
	// DB定義
	db := dynamo.New(sess)
	table := db.Table(Cnf.TableName)
	// 前回データの削除
	log.Print("前回記事の削除処理を開始します")
	deleteQiitaArticle(table, "overall")
	for _, tag := range Cnf.Tags {
		deleteQiitaArticle(table, tag)
	}
	log.Print("前回記事の削除処理が完了しました")
	// 記事取得日定義
	targetDate := dateNumToString(time.Now().AddDate(0, 0, Cnf.TargetDays))
	log.Print(targetDate, "以降の記事の取得を開始します")

	var datas []Data
	// 記事を蓄積
	log.Print("総合ランキングの取得を開始します")
	datas = addQiitaData(targetDate, Cnf.Stocks, "overall")
	if datas != nil && len(datas) != 0 {
		// 総合ランキングとして記事を登録
		log.Print("総合ランキングの登録を開始します")
		registQiitaData(table, datas, "overall")
	}
	log.Print("総合ランキングの登録が完了しました")

	// タグごとに記事を蓄積
	log.Print("タグごとの記事取得を開始します")
	for _, tag := range Cnf.Tags {
		datas = addQiitaData(targetDate, 0, tag)
		if datas != nil && len(datas) != 0 {
			// タグごとのランキングとして記事を登録
			log.Print("タグごとのランキング登録を開始します")
			registQiitaData(table, datas, tag)
		}
	}
	log.Print("処理を終了します")
}

func main() {
	// ラムダ実行
	lambda.Start(handler)
}
