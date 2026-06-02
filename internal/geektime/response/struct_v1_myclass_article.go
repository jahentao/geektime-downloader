package response

// V1MyClassArticleResponse ...
type V1MyClassArticleResponse struct {
	Code int `json:"code"`
	Data struct {
		ArticleID      int    `json:"article_id"`
		ArticleTitle   string `json:"article_title"`
		ArticleContent string `json:"article_content"`
		Type           int    `json:"type"`
		VideoID        string `json:"video_id"`
	} `json:"data"`
	Error struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	} `json:"error"`
}
