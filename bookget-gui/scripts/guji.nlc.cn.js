(function() {
	 // 获取最大页数
    let maxPages = 0;
		
	const div = document.querySelector('.container_read_body_image_bottom_num');
	if (div) {
	  const text = div.textContent;
	  const match = text.match(/\/(\d+)/); // 匹配 "*/数字"
	  if (match) {
		maxPages = parseInt(match[1]);
	  }
	}

    // 从localStorage获取当前执行次数，默认为0
    let executionCount = parseInt(localStorage.getItem('pageExecutionCount') || '0');
    
    // 如果已经达到或超过最大页数，直接返回
    if (executionCount >= maxPages) {
		localStorage.clear();
        return 'MAX_REACHED';
    }
    
    //自动点击：下一页
	const nextBtn = parent.querySelector('.container_read_body_image_bottom_arrow_rit');
    if (nextBtn) {
        nextBtn.click();
        
        // 增加执行计数并保存到localStorage
        executionCount++;
        localStorage.setItem('pageExecutionCount', executionCount.toString());
        
        return 'OK';
    }
    
    return '';
})();