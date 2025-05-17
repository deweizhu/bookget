const WORD_REGEX = /^[^//][^.]*$/;
const VALID_URI_REGEX = /^[-:.&#+()[\]$'*;@~!,?%=\/\w]+$/; // Will check that only RFC3986 allowed characters are included
const SCHEMED_URI_REGEX = /^\w+:.+$/;

let settings = {
    scriptsEnabled: true,
    blockPopups: true
};

const messageHandler = event => {
    var message = event.data.message;
    var args = event.data.args;

    switch (message) {
        case commands.MG_UPDATE_URI:
            if (isValidTabId(args.tabId)) {
                const tab = tabs.get(args.tabId);
                let previousURI = tab.uri;

                // Update the tab state
                tab.uri = args.uri;
                tab.uriToShow = args.uriToShow;
                tab.canGoBack = args.canGoBack;
                tab.canGoForward = args.canGoForward;

                // If the tab is active, update the controls UI
                if (args.tabId == activeTabId) {
                    updateNavigationUI(message);
                }

                isFavorite(tab.uri, (isFavorite) => {
                    tab.isFavorite = isFavorite;
                    updateFavoriteIcon();
                });

                // Don't add history entry if URI has not changed
                if (tab.uri == previousURI) {
                    break;
                }

                // Filter URIs that should not appear in history
                if (!tab.uri || tab.uri == 'about:blank') {
                    tab.historyItemId = INVALID_HISTORY_ID;
                    break;
                }

                if (tab.uriToShow && tab.uriToShow.substring(0, 10) == 'browser://') {
                    tab.historyItemId = INVALID_HISTORY_ID;
                    break;
                }

                addHistoryItem(historyItemFromTab(args.tabId), (id) => {
                    tab.historyItemId = id;
                });
            }
            break;
        case commands.MG_NAV_STARTING:
            if (isValidTabId(args.tabId)) {
                // Update the tab state
                tabs.get(args.tabId).isLoading = true;

                // If the tab is active, update the controls UI
                if (args.tabId == activeTabId) {
                    updateNavigationUI(message);
                }
            }
            break;
        case commands.MG_NAV_COMPLETED:
            if (isValidTabId(args.tabId)) {
                // Update tab state
                tabs.get(args.tabId).isLoading = false;

                // If the tab is active, update the controls UI
                if (args.tabId == activeTabId) {
                    updateNavigationUI(message);
                }
            }
            break;
        case commands.MG_UPDATE_TAB:
            if (isValidTabId(args.tabId)) {
                const tab = tabs.get(args.tabId);
                const tabElement = document.getElementById(`tab-${args.tabId}`);

                if (!tabElement) {
                    refreshTabs();
                    return;
                }

                // Update tab label
                // Use given title or fall back to a generic tab title
                tab.title = args.title || 'Tab';
                const tabLabel = tabElement.firstChild;
                const tabLabelSpan = tabLabel.firstChild;
                tabLabelSpan.textContent = tab.title;

                // Update title in history item
                // Browser pages will keep an invalid history ID
                if (tab.historyItemId != INVALID_HISTORY_ID) {
                    updateHistoryItem(tab.historyItemId, historyItemFromTab(args.tabId));
                }
            }
            break;
        case commands.MG_OPTIONS_LOST_FOCUS:
            let optionsButton = document.getElementById('btn-options');
            if (optionsButton) {
                if (optionsButton.className = 'btn-active') {
                    toggleOptionsDropdown();
                }
            }
            break;
        case commands.MG_SECURITY_UPDATE:
            if (isValidTabId(args.tabId)) {
                const tab = tabs.get(args.tabId);
                tab.securityState = args.state;

                if (args.tabId == activeTabId) {
                    updateNavigationUI(message);
                }
            }
            break;
        case commands.MG_UPDATE_FAVICON:
            if (isValidTabId(args.tabId)) {
                updateFaviconURI(args.tabId, args.uri);
            }
            break;
        case commands.MG_CLOSE_WINDOW:
            closeWindow();
            break;
        case commands.MG_CLOSE_TAB:
            if (isValidTabId(args.tabId)) {
                closeTab(args.tabId);
            }
            break;
        case commands.MG_GET_FAVORITES:
            if (isValidTabId(args.tabId)) {
                getFavoritesAsJson((payload) => {
                    args.favorites = payload;
                    window.chrome.webview.postMessage(event.data);
                });
            }
            break;
        case commands.MG_REMOVE_FAVORITE:
            removeFavorite(args.uri);
            break;
        case commands.MG_GET_SETTINGS:
            if (isValidTabId(args.tabId)) {
                args.settings = settings;
                window.chrome.webview.postMessage(event.data);
            }
            break;
        case commands.MG_GET_HISTORY:
            if (isValidTabId(args.tabId)) {
                getHistoryItems(args.from, args.count, (payload) => {
                    args.items = payload;
                    window.chrome.webview.postMessage(event.data);
                });
            }
            break;
        case commands.MG_REMOVE_HISTORY_ITEM:
            removeHistoryItem(args.id);
            break;
        case commands.MG_CLEAR_HISTORY:
            clearHistory();
            break;
        default:
            console.log(`Received unexpected message: ${JSON.stringify(event.data)}`);
    }
};

function processAddressBarInput() {
    var text = document.querySelector('#address-field').value;
    tryNavigate(text);
}

function tryNavigate(text) {
    try {
        var uriParser = new URL(text);

        // URL creation succeeded, verify protocol is allowed
        switch (uriParser.protocol) {
            case 'http:':
            case 'https:':
            case 'file:':
            case 'ftp:':
                // Allowed protocol, navigate
                navigateActiveTab(uriParser.href, false);
                break;
            default:
                // Protocol not allowed, search Bing
                navigateActiveTab(getSearchURI(text), true);
                break;
        }
    } catch (e) {
        // URL creation failed, check for invalid characters
        if (containsIlegalCharacters(text) || isSingleWord(text)) {
            // Search Bing
            navigateActiveTab(getSearchURI(text), true);
        } else {
            // Try with HTTP
            if (!hasScheme(text)) {
                tryNavigate(`http:${text}`);
            } else {
                navigateActiveTab(getSearchURI(text), true);
            }
        }
    }
}

function navigateActiveTab(uri, isSearch) {
    var message = {
        message: commands.MG_NAVIGATE,
        args: {
            uri: uri,
            encodedSearchURI: isSearch ? uri : getSearchURI(uri)
        }
    };

    window.chrome.webview.postMessage(message);
}

function reloadActiveTabContent() {
    var message = {
        message: commands.MG_RELOAD,
        args: {}
    };
    window.chrome.webview.postMessage(message);
}

function containsIlegalCharacters(query) {
    return !VALID_URI_REGEX.test(query);
}

function isSingleWord(query) {
    return WORD_REGEX.test(query);
}

function hasScheme(query) {
    return SCHEMED_URI_REGEX.test(query);
}

function getSearchURI(query) {
    return `https://www.bing.com/search?q=${encodeURIComponent(query)}`;
}

function closeWindow() {
    var message = {
        message: commands.MG_CLOSE_WINDOW,
        args: {}
    };

    window.chrome.webview.postMessage(message);
}

// Show active tab's URI in the address bar
function updateURI() {
    if (activeTabId == INVALID_TAB_ID) {
        return;
    }

    let activeTab = tabs.get(activeTabId);
    document.getElementById('address-field').value = activeTab.uriToShow || activeTab.uri;
}

// Show active tab's favicon in the address bar
function updateFavicon() {
    if (activeTabId == INVALID_TAB_ID) {
        return;
    }

    let activeTab = tabs.get(activeTabId);

    let faviconElement = document.getElementById('img-favicon');
    if (!faviconElement) {
        refreshControls();
        return;
    }

    faviconElement.src = activeTab.favicon;

}

// Update back and forward buttons for the active tab
function updateBackForwardButtons() {
    if (activeTabId == INVALID_TAB_ID) {
        return;
    }

    let activeTab = tabs.get(activeTabId);
    let btnForward = document.getElementById('btn-forward');
    let btnBack = document.getElementById('btn-back');

    if (!btnBack || !btnForward) {
        refreshControls();
        return;
    }

    if (activeTab.canGoForward)
        btnForward.className = 'btn';
    else
        btnForward.className = 'btn-disabled';

    if (activeTab.canGoBack)
        btnBack.className = 'btn';
    else
        btnBack.className = 'btn-disabled';
}

// Update reload button for the active tab
function updateReloadButton() {
    if (activeTabId == INVALID_TAB_ID) {
        return;
    }

    let activeTab = tabs.get(activeTabId);

    let btnReload = document.getElementById('btn-reload');
    if (!btnReload) {
        refreshControls();
        return;
    }

    btnReload.className = activeTab.isLoading ? 'btn-cancel' : 'btn';
}

// Update lock icon for the active tab
function updateLockIcon() {
    if (activeTabId == INVALID_TAB_ID) {
        return;
    }

    let activeTab = tabs.get(activeTabId);

    let labelElement = document.getElementById('security-label');
    if (!labelElement) {
        refreshControls();
        return;
    }

    switch (activeTab.securityState) {
        case 'insecure':
            labelElement.className = 'label-insecure';
            break;
        case 'neutral':
            labelElement.className = 'label-neutral';
            break;
        case 'secure':
            labelElement.className = 'label-secure';
            break;
        default:
            labelElement.className = 'label-unknown';
            break;
    }
}

// Update favorite status for the active tab
function updateFavoriteIcon() {
    if (activeTabId == INVALID_TAB_ID) {
        return;
    }

    let activeTab = tabs.get(activeTabId);
    isFavorite(activeTab.uri, (isFavorite) => {
        let favoriteElement = document.getElementById('btn-fav');
        if (!favoriteElement) {
            refreshControls();
            return;
        }

        if (isFavorite) {
            favoriteElement.classList.add('favorited');
            activeTab.isFavorite = true;
        } else {
            favoriteElement.classList.remove('favorited');
            activeTab.isFavorite = false;
        }
    });

}

function updateNavigationUI(reason) {
    switch (reason) {
        case commands.MG_UPDATE_URI:
            updateURI();
            updateFavoriteIcon();
            updateBackForwardButtons();
            break;
        case commands.MG_NAV_COMPLETED:
        case commands.MG_NAV_STARTING:
            updateReloadButton();
            break;
        case commands.MG_SECURITY_UPDATE:
            updateLockIcon();
            break;
        case commands.MG_UPDATE_FAVICON:
            updateFavicon();
            break;
        // If a reason is not provided (for requests not originating from a
        // message), default to switch tab behavior.
        default:
        case commands.MG_SWITCH_TAB:
            updateURI();
            updateLockIcon();
            updateFavicon();
            updateFavoriteIcon();
            updateReloadButton();
            updateBackForwardButtons();
            break;
    }
}

function loadTabUI(tabId) {
    if (isValidTabId(tabId)) {
        let tab = tabs.get(tabId);

        let tabElement = document.createElement('div');
        tabElement.className = tabId == activeTabId ? 'tab-active' : 'tab';
        tabElement.id = `tab-${tabId}`;

        let tabLabel = document.createElement('div');
        tabLabel.className = 'tab-label';

        let labelText = document.createElement('span');
        labelText.textContent = tab.title;
        tabLabel.appendChild(labelText);

        let closeButton = document.createElement('div');
        closeButton.className = 'btn-tab-close';
        closeButton.addEventListener('click', function(e) {
            closeTab(tabId);
        });

        tabElement.appendChild(tabLabel);
        tabElement.appendChild(closeButton);

        var createTabButton = document.getElementById('btn-new-tab');
        document.getElementById('tabs-strip').insertBefore(tabElement, createTabButton);

        tabElement.addEventListener('click', function(e) {
            if (e.srcElement.className != 'btn-tab-close') {
                switchToTab(tabId, true);
            }
        });
    }
}

function toggleOptionsDropdown() {
    const optionsButtonElement = document.getElementById('btn-options');
    const elementClass = optionsButtonElement.className;

    var message;
    if (elementClass === 'btn') {
        // Update UI
        optionsButtonElement.className = 'btn-active';

        message = {
            message: commands.MG_SHOW_OPTIONS,
            args: {}
        };
    } else {
        // Update UI
        optionsButtonElement.className = 'btn';

        message = {
            message:commands.MG_HIDE_OPTIONS,
            args: {}
        };
    }

    window.chrome.webview.postMessage(message);
}

function refreshControls() {
    let controlsElement = document.getElementById('controls-bar');
    if (controlsElement) {
        controlsElement.remove();
    }

    controlsElement = document.createElement('div');
    controlsElement.id = 'controls-bar';

    // Navigation controls
    let navControls = document.createElement('div');
    navControls.className = 'controls-group';
    navControls.id = 'nav-controls-container';

    let backButton = document.createElement('div');
    backButton.className = 'btn-disabled';
    backButton.id = 'btn-back';
    navControls.append(backButton);

    let forwardButton = document.createElement('div');
    forwardButton.className = 'btn-disabled';
    forwardButton.id = 'btn-forward';
    navControls.append(forwardButton);

    let reloadButton = document.createElement('div');
    reloadButton.className = 'btn';
    reloadButton.id = 'btn-reload';
    navControls.append(reloadButton);

    controlsElement.append(navControls);

    // Address bar
    let addressBar = document.createElement('div');
    addressBar.id = 'address-bar-container';

    let securityLabel = document.createElement('div');
    securityLabel.className = 'label-unknown';
    securityLabel.id = 'security-label';

    let labelSpan = document.createElement('span');
    labelSpan.textContent = 'Not secure';
    securityLabel.append(labelSpan);

    let lockIcon = document.createElement('div');
    lockIcon.className = 'icn';
    lockIcon.id = 'icn-lock';
    securityLabel.append(lockIcon);
    addressBar.append(securityLabel);

    let faviconElement = document.createElement('div');
    faviconElement.className = 'icn';
    faviconElement.id = 'icn-favicon';

    let faviconImage = document.createElement('img');
    faviconImage.id = 'img-favicon';
    faviconImage.src = 'img/favicon.png';
    faviconElement.append(faviconImage);
    addressBar.append(faviconElement);

    let addressInput = document.createElement('input');
    addressInput.id = 'address-field';
    addressInput.placeholder = 'Search or enter web address';
    addressInput.type = 'text';
    addressInput.spellcheck = false;
    addressBar.append(addressInput);

    let clearButton = document.createElement('button');
    clearButton.id = 'btn-clear';
    addressBar.append(clearButton);

    let favoriteButton = document.createElement('div');
    favoriteButton.className = 'icn';
    favoriteButton.id = 'btn-fav';
    addressBar.append(favoriteButton);
    controlsElement.append(addressBar);

    // Manage controls
    let manageControls = document.createElement('div');
    manageControls.className = 'controls-group';
    manageControls.id = 'manage-controls-container';

    let optionsButton = document.createElement('div');
    optionsButton.className = 'btn';
    optionsButton.id = 'btn-options';
    manageControls.append(optionsButton);
    controlsElement.append(manageControls);

    // Insert controls bar into document
    let tabsElement = document.getElementById('tabs-strip');
    if (tabsElement) {
        tabsElement.parentElement.insertBefore(controlsElement, tabsElement);
    } else {
        let bodyElement = document.getElementsByTagName('body')[0];
        bodyElement.append(controlsElement);
    }

    addControlsListeners();
    updateNavigationUI();
}

function refreshTabs() {
    let tabsStrip = document.getElementById('tabs-strip');
    if (tabsStrip) {
        tabsStrip.remove();
    }

    tabsStrip = document.createElement('div');
    tabsStrip.id = 'tabs-strip';

    let newTabButton = document.createElement('div');
    newTabButton.id = 'btn-new-tab';

    let buttonSpan = document.createElement('span');
    buttonSpan.textContent = '+';
    buttonSpan.id = 'plus-label';
    newTabButton.append(buttonSpan);
    tabsStrip.append(newTabButton);

    let bodyElement = document.getElementsByTagName('body')[0];
    bodyElement.append(tabsStrip);

    addTabsListeners();

    Array.from(tabs).map((tabEntry) => {
        loadTabUI(tabEntry[0]);
    });
}

function toggleFavorite() {
    activeTab = tabs.get(activeTabId);
    if (activeTab.isFavorite) {
        removeFavorite(activeTab.uri, () => {
            activeTab.isFavorite = false;
            updateFavoriteIcon();
        });
    } else {
        addFavorite(favoriteFromTab(activeTabId), () => {
            activeTab.isFavorite = true;
            updateFavoriteIcon();
        });
    }
}

function addControlsListeners() {
    let inputField = document.querySelector('#address-field');
    let clearButton = document.querySelector('#btn-clear');

    inputField.addEventListener('keypress', function(e) {
        var key = e.which || e.keyCode;
        if (key === 13) { // 13 is enter
            e.preventDefault();
            processAddressBarInput();
        }
    });

    inputField.addEventListener('focus', function(e) {
        e.target.select();
    });

    inputField.addEventListener('blur', function(e) {
        inputField.setSelectionRange(0, 0);
        if (!inputField.value) {
            updateURI();
        }
    });

    clearButton.addEventListener('click', function(e) {
        inputField.value = '';
        inputField.focus();
        e.preventDefault();
    });

    document.querySelector('#btn-forward').addEventListener('click', function(e) {
        if (document.getElementById('btn-forward').className === 'btn') {
            var message = {
                message: commands.MG_GO_FORWARD,
                args: {}
            };
            window.chrome.webview.postMessage(message);
        }
    });

    document.querySelector('#btn-back').addEventListener('click', function(e) {
        if (document.getElementById('btn-back').className === 'btn') {
            var message = {
                message: commands.MG_GO_BACK,
                args: {}
            };
            window.chrome.webview.postMessage(message);
        }
    });

    document.querySelector('#btn-reload').addEventListener('click', function(e) {
        var btnReload = document.getElementById('btn-reload');
        if (btnReload.className === 'btn-cancel') {
            var message = {
                message: commands.MG_CANCEL,
                args: {}
            };
            window.chrome.webview.postMessage(message);
        } else if (btnReload.className === 'btn') {
            reloadActiveTabContent();
        }
    });

    document.querySelector('#btn-options').addEventListener('click', function(e) {
        toggleOptionsDropdown();
    });

    window.onkeydown = function(event) {
        if (event.ctrlKey) {
            switch (event.key) {
                case 'r':
                case 'R':
                    reloadActiveTabContent();
                    break;
                case 'd':
                case 'D':
                    toggleFavorite();
                    break;
                case 't':
                case 'T':
                    createNewTab(true);
                    break;
                case 'p':
                case 'P':
                case '+':
                case '-':
                case '_':
                case '=':
                    break;
                default:
                    return;
            }

            event.preventDefault();
        }
    };

    // Prevent zooming the UI
    window.addEventListener('wheel', function(event) {
        if (event.ctrlKey) {
            event.preventDefault();
        }}, { passive: false });
}

function addTabsListeners() {
    document.querySelector('#btn-new-tab').addEventListener('click', function(e) {
        createNewTab(true);
    });

    document.querySelector('#btn-fav').addEventListener('click', function(e) {
        toggleFavorite();
    });
}

function init() {
    window.chrome.webview.addEventListener('message', messageHandler);
    refreshControls();
    refreshTabs();

    createNewTab(true);
}

init();
