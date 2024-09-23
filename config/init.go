package config

import (
	"os"
	"strconv"
	"strings"
)

var Conf Input

const version = "24.0923"

// initSeq    false = 最小值 <= 当前页码 <=  最大值
func initSeqRange() {
	if Conf.Seq == "" || !strings.Contains(Conf.Seq, ":") {
		return
	}
	m := strings.Split(Conf.Seq, ":")
	if len(m) == 1 {
		Conf.SeqStart, _ = strconv.Atoi(m[0])
		Conf.SeqEnd = Conf.SeqStart
	} else {
		Conf.SeqStart, _ = strconv.Atoi(m[0])
		Conf.SeqEnd, _ = strconv.Atoi(m[1])
	}
	return
}

// initVolumeRange    false = 最小值 <= 当前页码 <=  最大值
func initVolumeRange() {
	m := strings.Split(Conf.Volume, ":")
	if len(m) == 1 {
		Conf.VolStart, _ = strconv.Atoi(m[0])
		Conf.VolEnd = Conf.VolStart
	} else {
		Conf.VolStart, _ = strconv.Atoi(m[0])
		Conf.VolEnd, _ = strconv.Atoi(m[1])
	}
	return
}

func UserHomeDir() string {
	if os.PathSeparator == '\\' {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func UserTmpDir() string {
	if os.PathSeparator == '\\' {
		return UserHomeDir() + "\\AppData\\Roaming\\BookGet\\bookget\\User Data\\"
	}
	return UserHomeDir() + "/bookget/"
}
