(function () {

    const div = document.querySelector('.container_read_body_image_bottom_num');
    if (!div) {
        return 'MAX_PAGE';
    }
    const text = div.textContent;
    const match = text.match(/\/(\d+)/); // 匹配 "*/数字"
    if (!match) {
        return 'MAX_PAGE';
    }
    // 获取最大页数
    const currentIndex = parseInt(match[0]);
    const maxPages = parseInt(match[1]);
    console.log(maxPages)
    if (currentIndex >= maxPages) {
        return 'MAX_REACHED';
    }
    //自动点击：下一页
    const nextBtn = document.querySelector('.container_read_body_image_bottom_arrow_rit');
    console.log(nextBtn)
    if (nextBtn) {
        nextBtn.click();
        console.log('点击成功')
        return 'OK';
    }
    console.log('失击失败')
    return '';
})();