package sillokgokr

import (
	"strconv"
	"strings"
)

type Canvases struct {
	KingCode   string `json:"kingCode"`
	ImageId    string `json:"imageId"`
	Previous   string `json:"previous"`
	Firstchild string `json:"firstchild"`
	Title      string `json:"title"`
	PageId     string `json:"pageId"`
	Parent     string `json:"parent"`
	Type       string `json:"type"`
	Level      string `json:"level"`
	Next       string `json:"next"`
	Seq        string `json:"seq"`
}

type Response struct {
	TreeList struct {
		List      []Canvases `json:"list"`
		ListCount int        `json:"listCount"`
	} `json:"treeList"`
}

type Book struct {
	Title      string
	Seq        int
	Id         string
	KingCode   string
	Level      int
	Type       string
	Next       string
	Firstchild string
}

type ByImageIdSort []Canvases

func (ni ByImageIdSort) Len() int      { return len(ni) }
func (ni ByImageIdSort) Swap(i, j int) { ni[i], ni[j] = ni[j], ni[i] }
func (ni ByImageIdSort) Less(i, j int) bool {
	idA := ni[i].ImageId
	idB := ni[j].ImageId
	a, err1 := strconv.ParseInt(idA[strings.LastIndex(idA, "_"):], 10, 64)
	b, err2 := strconv.ParseInt(idB[strings.LastIndex(idB, "_"):], 10, 64)
	if err1 != nil || err2 != nil {
		return idA < idB
	}
	return a < b
}
