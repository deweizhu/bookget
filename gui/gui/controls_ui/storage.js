function handleUpgradeEvent(event) {
    console.log('Creating DB');
    let newDB = event.target.result;

    newDB.onerror = function(event) {
        console.log('Something went wrong');
        console.log(event);
    };

    let newFavoritesStore = newDB.createObjectStore('favorites', {
        keyPath: 'uri'
    });

    newFavoritesStore.transaction.oncomplete = function(event) {
        console.log('Object stores created');
    };

    let newHistoryStore = newDB.createObjectStore('history', {
        autoIncrement: true
    });

    newHistoryStore.createIndex('timestamp', 'timestamp', {
        unique: false
    });

    newHistoryStore.createIndex('stampedURI', ['uri', 'timestamp'], {
        unique: false
    });
}

function queryDB(query) {
    let request = window.indexedDB.open('WVBrowser');

    request.onerror = function(event) {
        console.log('Failed to open database');
    };

    request.onsuccess = function(event) {
        let db = event.target.result;
        query(db);
    };

    request.onupgradeneeded = handleUpgradeEvent;
}
