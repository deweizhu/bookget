function navigateToBrowserPage(path) {
    const navMessage = {
        message: commands.MG_NAVIGATE,
        args: {
            uri: `browser://${path}`
        }
    };

    window.chrome.webview.postMessage(navMessage);
}

// Add listener for the options menu entries
function addItemsListeners() {

    // Close dropdown when item is selected
    (() => {
        const dropdownItems = Array.from(document.getElementsByClassName('dropdown-item'));
        dropdownItems.map(item => {
            item.addEventListener('click', function(e) {
                const closeMessage = {
                    message: commands.MG_OPTION_SELECTED,
                    args: {}
                };
                window.chrome.webview.postMessage(closeMessage);
            });

            // Navigate to browser page
            let entry = item.id.split('-')[1];
            switch (entry) {
                case 'settings':
                case 'history':
                case 'favorites':
                    item.addEventListener('click', function(e) {
                        navigateToBrowserPage(entry);
                    });
                    break;
            }
        });
    })();
}

function init() {
    addItemsListeners();
}

init();
