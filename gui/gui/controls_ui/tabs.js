var tabs = new Map();
var tabIdCounter = 0;
var activeTabId = 0;
const INVALID_TAB_ID = 0;

function getNewTabId() {
    return ++tabIdCounter;
}

function isValidTabId(tabId) {
    return tabId != INVALID_TAB_ID && tabs.has(tabId);
}

function createNewTab(shouldBeActive) {
    const tabId = getNewTabId();

    var message = {
        message: commands.MG_CREATE_TAB,
        args: {
            tabId: parseInt(tabId),
            active: shouldBeActive || false
        }
    };

    window.chrome.webview.postMessage(message);

    tabs.set(parseInt(tabId), {
        title: 'New Tab',
        uri: '',
        uriToShow: '',
        favicon: 'img/favicon.png',
        isFavorite: false,
        isLoading: false,
        canGoBack: false,
        canGoForward: false,
        securityState: 'unknown',
        historyItemId: INVALID_HISTORY_ID
    });

    loadTabUI(tabId);

    if (shouldBeActive) {
        switchToTab(tabId, false);
    }
}

function switchToTab(id, updateOnHost) {
    if (!id) {
        console.log('ID not provided');
        return;
    }

    // Check the tab to switch to is valid
    if (!isValidTabId(id)) {
        return;
    }

    // No need to switch if the tab is already active
    if (id == activeTabId) {
        return;
    }

    // Get the tab element to switch to
    var tab = document.getElementById(`tab-${id}`);
    if (!tab) {
        console.log(`Can't switch to tab ${id}: element does not exist`);
        return;
    }

    // Change the style for the previously active tab
    if (isValidTabId(activeTabId)) {
        const activeTabElement = document.getElementById(`tab-${activeTabId}`);

        // Check the previously active tab element does actually exist
        if (activeTabElement) {
            activeTabElement.className = 'tab';
        }
    }

    // Set tab as active
    tab.className = 'tab-active';
    activeTabId = id;

    // Instruct host app to switch tab
    if (updateOnHost) {
        var message = {
            message: commands.MG_SWITCH_TAB,
            args: {
                tabId: parseInt(activeTabId)
            }
        };

        window.chrome.webview.postMessage(message);
    }

    updateNavigationUI(commands.MG_SWITCH_TAB);
}

function closeTab(id) {
    // If closing tab was active, switch tab or close window
    if (id == activeTabId) {
        if (tabs.size == 1) {
            // Last tab is closing, shut window down
            tabs.delete(id);
            closeWindow();
            return;
        }

        // Other tabs are open, switch to rightmost tab
        var tabsEntries = Array.from(tabs.entries());
        var lastEntry = tabsEntries.pop();
        if (lastEntry[0] == id) {
            lastEntry = tabsEntries.pop();
        }
        switchToTab(lastEntry[0], true);
    }

    // Remove tab element
    var tabElement = document.getElementById(`tab-${id}`);
    if (tabElement) {
        tabElement.parentNode.removeChild(tabElement);
    }
    // Remove tab from map
    tabs.delete(id);

    var message = {
        message: commands.MG_CLOSE_TAB,
        args: {
            tabId: id
        }
    };

    window.chrome.webview.postMessage(message);
}

function updateFaviconURI(tabId, src) {
    let tab = tabs.get(tabId);
    if (tab.favicon != src) {
        let img = new Image();

        // Update favicon element on successful load
        img.onload = () => {
            console.log('Favicon loaded');
            tab.favicon = src;

            if (tabId == activeTabId) {
                updatedFaviconURIHandler(tabId, tab);
            }
        };

        if (src) {
            // Try load from root on failed load
            img.onerror = () => {
                console.log('Cannot load favicon from link, trying with root');
                updateFaviconURI(tabId, '');
            };
        } else {
            // No link for favicon, try loading from root
            try {
                let tabURI = new URL(tab.uri);
                src = `${tabURI.origin}/favicon.ico`;
            } catch(e) {
                console.log(`Could not parse tab ${tabId} URI`);
            }

            img.onerror = () => {
                console.log('No favicon in site root. Using default favicon.');
                tab.favicon = 'img/favicon.png';
                updatedFaviconURIHandler(tabId, tab);
            };
        }

        img.src = src;
    }
}

function updatedFaviconURIHandler(tabId, tab) {
    updateNavigationUI(commands.MG_UPDATE_FAVICON);

    // Update favicon in history item
    if (tab.historyItemId != INVALID_HISTORY_ID) {
        updateHistoryItem(tab.historyItemId, historyItemFromTab(tabId));
    }
}

function favoriteFromTab(tabId) {
    if (!isValidTabId(tabId)) {
        console.log('Invalid tab ID');
        return;
    }

    let tab = tabs.get(tabId);
    let favicon = tab.favicon == 'img/favicon.png' ? '../controls_ui/' + tab.favicon : tab.favicon;
    return {
        uri: tab.uri,
        uriToShow: tab.uriToShow,
        title: tab.title,
        favicon: favicon
    };
}

function historyItemFromTab(tabId) {
    if (!isValidTabId(tabId)) {
        console.log('Invalid tab ID');
        return;
    }

    let tab = tabs.get(tabId);
    let favicon = tab.favicon == 'img/favicon.png' ? '../controls_ui/' + tab.favicon : tab.favicon;
    return {
        uri: tab.uri,
        title: tab.title,
        favicon: favicon,
        timestamp: new Date()
    }
}
