(function () {

    const pages = document.querySelectorAll('.container_read_head_body_image_footer_paging_font');
	const currentPage = pages[0].value || pages[0].textContent;
	const totalPages = pages[1].value || pages[1].textContent;
    // 获取最大页数
    const currentIndex = parseInt(currentPage);
    const maxPages = parseInt(totalPages);
    if (currentIndex >= maxPages) {
        return 'MAX_REACHED';
    }
    //自动点击：下一页
    const arrow = document.querySelectorAll('.container_read_head_body_image_footer_paging_arrow');
    const nextBtn = arrow[1];
    console.log(nextBtn)
    if (nextBtn) {
        nextBtn.click();
        console.log('点击成功')
        return 'OK';
    }
    console.log('失击失败')
    return '';
})();