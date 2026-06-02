package response

// V1MyProductsResponse ...
type V1MyProductsResponse struct {
	Data []V1MyProductCategory `json:"data"`
	Code int                   `json:"code"`
}

// V1ColumnNewAllResponse ...
type V1ColumnNewAllResponse struct {
	Data struct {
		List []V1ColumnNewAllItem `json:"list"`
	} `json:"data"`
	Code int `json:"code"`
}

// V1ColumnNewAllItem ...
type V1ColumnNewAllItem struct {
	ID           int    `json:"id"`
	ColumnSKU    int    `json:"column_sku"`
	ColumnType   int    `json:"column_type"`
	HadSub       bool   `json:"had_sub"`
	IsRealSub    bool   `json:"is_real_sub"`
	IsVIP        bool   `json:"is_vip"`
	ArticleCount int    `json:"article_count"`
}

// V1MyProductCategory ...
type V1MyProductCategory struct {
	ID    int                `json:"id"`
	Title string             `json:"title"`
	Page  struct {
		More  bool `json:"more"`
		Count int  `json:"count"`
	} `json:"page"`
	List []V1MyProductItem `json:"list"`
}

// V1MyProductItem ...
type V1MyProductItem struct {
	Title string `json:"title"`
	Cover string `json:"cover"`
	Type  string `json:"type"`
	Extra struct {
		LastAid        int    `json:"last_aid"`
		ColumnID       int    `json:"column_id"`
		ColumnTitle    string `json:"column_title"`
		ColumnSubtitle string `json:"column_subtitle"`
		AuthorName     string `json:"author_name"`
		AuthorIntro    string `json:"author_intro"`
		ColumnCover    string `json:"column_cover"`
		ColumnType     int    `json:"column_type"`
		ArticleCount   int    `json:"article_count"`
		IsIncludeAudio bool   `json:"is_include_audio"`
	} `json:"extra"`
}
