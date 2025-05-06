package util

type SortByStr []string

func (s SortByStr) Len() int           { return len(s) }
func (s SortByStr) Less(i, j int) bool { return s[i] < s[j] }
func (s SortByStr) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
