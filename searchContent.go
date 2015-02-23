/**
 * This package contains the functions to obtain tweets through Twitter API. Return tweets will be clustered
 * by content. Similar content tweets will be in same group with same group id.
 * The structure defined here are selected. Other information can be added if it exists query return result.
 *
 * This can be simply type following command to get clutered information.
 *
 * go run search.go -q=KEYWORDS
 *
 */
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type ResponseType struct {
	TokenType   string `json:"token_type"`
	AccessToken string `json:"access_token"`
}

type User struct {
	Id         int64  `json:"id"`
	IdStr      string `json:"id_str"`
	Name       string `json:"name"`
	ScreenName string `json:"screen_name"`
}

type Tweet struct {
	CreatedAt     string `json:"created_at"`
	FavoriteCount int    `json:"favorite_count"`
	Text          string `json:"text"`
	User          User   `json:"user"`
}

type Metadata struct {
	CompletedIn   float32 `json:"completed_in"`
	MaxId         int64   `json:"max_id"`
	MaxIdString   string  `json:"max_id_str"`
	Query         string  `json:"query"`
	RefreshUrl    string  `json:"refresh_url"`
	Count         int     `json:"count"`
	SinceId       int64   `json:"since_id"`
	SinceIdString string  `json:"since_id_str"`
}

type SearchResponse struct {
	Tweets   []Tweet  `json:"statuses"`
	Metadata Metadata `json:"search_metadata"`
}

type ResultElement struct {
	OneTweet Tweet
	Vector   map[string]float64
}

// Global variables
var Dict = make(map[string]float64)
var Correlation = float64(0.5)
var Count int = 0

/**
 * Get basic token via key and serect
 *
 */

func GetBasicToken(key string, secret string) string {
	credential := key + ":" + secret
	token := base64.StdEncoding.EncodeToString([]byte(credential))
	return token
}

/**
 * Get bear token
 *
 * Get bear token via sending request to Twitter API
 *
 */

func GetBearerToken(basicToken string) string {
	authUrl := "https://api.twitter.com/oauth2/token"
	client := &http.Client{}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, _ := http.NewRequest("POST", authUrl, bytes.NewBufferString(data.Encode()))
	req.Header.Add("Authorization", "Basic "+basicToken)
	req.Header.Add("Content-type", "application/x-www-form-urlencoded;charset=UTF-8")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("error when getting bearer token")
		os.Exit(1)
	}
	var rt ResponseType
	if resp.StatusCode == 200 {
		bodyByte, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal("error when retrieve response body")
			os.Exit(1)
		}
		_ = json.Unmarshal(bodyByte, &rt)
	} else {
		log.Fatal("Cannot get bearer token as status code is not 200")
		os.Exit(1)
	}
	return rt.AccessToken
}

/**
 * Get query result
 *
 * Get the query result through Twitter API
 * Output will be a structure contain twitter information parsed from return result
 *
 */

func GetQueryResults(bearerToken string, searchWord string) SearchResponse {
	searchUrl, err := url.Parse("https://api.twitter.com/1.1/search/tweets.json")
	if err != nil {
		log.Fatal("Cannot parser search url")
		os.Exit(1)
	}

	client := &http.Client{}

	data := url.Values{}
	data.Set("q", searchWord)
	searchUrl.RawQuery = data.Encode()

	req, _ := http.NewRequest("GET", searchUrl.String(), nil)
	req.Header.Add("Authorization", "Bearer "+bearerToken)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("error when getting bearer token")
		os.Exit(1)
	}

	var sr SearchResponse
	if resp.StatusCode == 200 {
		bodyByte, err := ioutil.ReadAll(resp.Body)
		if err != nil {
		}
		_ = json.Unmarshal(bodyByte, &sr)
	} else {
		log.Fatal("Cannot get results as status code is not 200")
		os.Exit(1)
	}
	return sr
}

func MakeDict(sr SearchResponse) {
	for i := 1; i < len(sr.Tweets); i++ {
		words := strings.Fields(sr.Tweets[i].Text)
		for i := range words {
			Dict[words[i]] = float64(0)
		}
	}
}

func Distance(gDict, lDict map[string]float64) float64 {
	if len(gDict) != len(lDict) {
		log.Fatal("Vector size does not match!\n")
		os.Exit(1)
	}
	var Divd, Divs1, Divs2 float64 = 0, 0, 0

	keys := make([]string, len(Dict))
	for k := range gDict {
		keys = append(keys, k)
	}
	for i := range keys {
		Divd += gDict[keys[i]] * lDict[keys[i]]
		Divs1 += gDict[keys[i]] * gDict[keys[i]]
		Divs2 += lDict[keys[i]] * lDict[keys[i]]
	}

	return Divd / (math.Sqrt(Divs1) * math.Sqrt(Divs2))
}

func CalculateRelation(sr SearchResponse) map[int][]ResultElement {
	var resultMap = make(map[int][]ResultElement)
OuterLoop:
	for i := 1; i < len(sr.Tweets); i++ {
		words := strings.Fields(sr.Tweets[i].Text)
		var LocalElement ResultElement
		LocalElement.OneTweet = sr.Tweets[i]
		LocalElement.Vector = make(map[string]float64)
		for k, v := range Dict {
			LocalElement.Vector[k] = v
		}
		for i := range words {
			LocalElement.Vector[words[i]] = float64(1.0)
		}

		for j := 0; j < Count; j++ {
			for k := 0; k < len(resultMap[j]); k++ {
				if v := Distance(resultMap[j][k].Vector, LocalElement.Vector); v > Correlation {
					resultMap[k] = append(resultMap[k], LocalElement)
					continue OuterLoop
				}
			}
		}
		resultMap[Count] = append(resultMap[Count], LocalElement)
		Count += 1
	}

	return resultMap
}

/**
 * Clustering function
 *
 * Simply cluster the tweets by user.
 */

func Cluster(sr SearchResponse) map[int][]ResultElement {
	MakeDict(sr)
	return CalculateRelation(sr)
}

/**
 * Printing out the Result
 *
 * Print the result out with group id, user's screen name, creation time and the tweet content.
 * Since the return result is ordered by time, the result does not need to be reordered.
 * Tab is used to separate each element for a tweet. And a "#" with new line will separate the different elements
 */

func PrintResult(m map[int][]ResultElement) {
	for i := 0; i < Count; i++ {
		for j := 0; j < len(m[i]); j++ {
			var pResult string = strconv.Itoa(i) + "\t" +
				m[i][j].OneTweet.User.ScreenName + "\t" +
				m[i][j].OneTweet.CreatedAt + "\t" +
				strings.Replace(m[i][j].OneTweet.Text, "\n", " ", -1) + "\n"
			fmt.Printf(pResult)
		}
		fmt.Printf("#\n")
	}
}

func main() {
	searchWord := flag.String("q", "", "search strings")
	key := flag.String("key", "", "Twitter API Key")
	secret := flag.String("secrect", "", "Twitter API Secrect")
	bearerToken := flag.String("bearer_token", "AAAAAAAAAAAAAAAAAAAAAGhReAAAAAAAXj%2FoYD0GDcUNnUz8OGtXP%2Bim9pY%3DIR1J1wMpR8AKtnJ2Ooo3SCStwUTVMLRj3CKT9BnVn455LSh42s", "Twitter API BearerToken")
	flag.Parse()

	// Since the key and secret are private ones. I hide them and direct hardcode the bearerToken.
	// You can replace the key and secret by following code and enable the bearToken generation.
	// Code :
	//

	if *key != "" || *secret != "" {
		basicToken := GetBasicToken(*key, *secret)
		*bearerToken = GetBearerToken(basicToken)
	}

	sr := GetQueryResults(*bearerToken, *searchWord)
	resultMap := Cluster(sr)
	PrintResult(resultMap)
}
