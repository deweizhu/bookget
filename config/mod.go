package config

// PageRange    return true (最小值 <= 当前页码 <=  最大值)
func PageRange(index, size int) bool {
	//未设置
	if Conf.SeqStart <= 0 {
		return true
	}
	//结束页负数
	if Conf.SeqEnd < 0 && (index-size >= Conf.SeqEnd) {
		return false
	}
	//结束页
	if Conf.SeqEnd > 0 {
		//结束了
		if index >= Conf.SeqEnd {
			return false
		}
		//起始页
		if index+1 >= Conf.SeqStart {
			return true
		}
	} else if index+1 >= Conf.SeqStart { //在起始页后
		return true
	}
	return false
}

// VolumeRange    return true (最小值 <= 当前页码 <=  最大值)
func VolumeRange(index int) bool {
	//未设置
	if Conf.VolStart <= 0 {
		return true
	}
	//结束页负数
	if Conf.VolEnd < 0 && index > Conf.VolStart {
		return false
	}
	//结束页
	if Conf.VolEnd > 0 {
		//结束了
		if index >= Conf.VolEnd {
			return false
		}
		//起始页
		if index+1 >= Conf.VolStart {
			return true
		}
	} else if index+1 >= Conf.VolStart { //在起始页后
		return true
	}
	return false
}
