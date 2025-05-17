const messageHandler = event => {
    var message = event.data.message;
    var args = event.data.args;

    switch (message) {
        case commands.MG_GET_SETTINGS:
            loadSettings(args.settings);
            break;
        case commands.MG_CLEAR_CACHE:
            if (args.content && args.controls) {
                updateLabelForEntry('entry-cache', 'Cleared');
            } else {
                updateLabelForEntry('entry-cache', 'Try again');
            }
            break;
        case commands.MG_CLEAR_COOKIES:
            if (args.content && args.controls) {
                updateLabelForEntry('entry-cookies', 'Cleared');
            } else {
                updateLabelForEntry('entry-cookies', 'Try again');
            }
            break;
        default:
            console.log(`Unexpected message: ${JSON.stringify(event.data)}`);
            break;
    }
};

function addEntriesListeners() {
    let cacheEntry = document.getElementById('entry-cache');
    cacheEntry.addEventListener('click', function(e) {
        let message = {
            message: commands.MG_CLEAR_CACHE,
            args: {}
        };

        window.chrome.webview.postMessage(message);
    });

    let cookiesEntry = document.getElementById('entry-cookies');
    cookiesEntry.addEventListener('click', function(e) {
        let message = {
            message: commands.MG_CLEAR_COOKIES,
            args: {}
        };

        window.chrome.webview.postMessage(message);
    });

    let scriptEntry = document.getElementById('entry-script');
    scriptEntry.addEventListener('click', function(e) {
        // Toggle script support
    });

    let popupsEntry = document.getElementById('entry-popups');
    popupsEntry.addEventListener('click', function(e) {
        // Toggle popups
    });
}

function requestBrowserSettings() {
    let message = {
        message: commands.MG_GET_SETTINGS,
        args: {}
    };

    window.chrome.webview.postMessage(message);
}

function loadSettings(settings) {
    if (settings.scriptsEnabled) {
        updateLabelForEntry('entry-script', 'Enabled');
    } else {
        updateLabelForEntry('entry-script', 'Disabled');
    }

    if (settings.blockPopups) {
        updateLabelForEntry('entry-popups', 'Blocked');
    } else {
        updateLabelForEntry('entry-popups', 'Allowed');
    }
}

function updateLabelForEntry(elementId, label) {
    let entryElement = document.getElementById(elementId);
    if (!entryElement) {
        console.log(`Element with id ${elementId} does not exist`);
        return;
    }

    let labelSpan = entryElement.querySelector(`.entry-value span`);

    if (!labelSpan) {
        console.log(`${elementId} does not have a label`);
        return;
    }

    labelSpan.textContent = label;
}

function init() {
    window.chrome.webview.addEventListener('message', messageHandler);
    requestBrowserSettings();
    addEntriesListeners();
}

init();
