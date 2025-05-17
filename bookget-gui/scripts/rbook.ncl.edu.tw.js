(function() {
    // 获取最大页数
    const maxPages = document.getElementById('sel-content-no') ? 
        document.getElementById('sel-content-no').options.length : 0;
    
    // 从localStorage获取当前执行次数，默认为0
    let executionCount = parseInt(localStorage.getItem('pageExecutionCount') || '0');
    
    // 如果已经达到或超过最大页数，直接返回
    if (executionCount >= maxPages) {
		localStorage.clear();
        return 'MAX_REACHED';
    }
    
    //自动点击：下一页
    if (document.getElementById('AftT')) {
        document.getElementById('AftT').click();
        
        // 增加执行计数并保存到localStorage
        executionCount++;
        localStorage.setItem('pageExecutionCount', executionCount.toString());
        
        return 'OK';
    }
    
    return '';
})();