const INVALID_HISTORY_ID = -1;

function addHistoryItem(item, callback) {
    queryDB((db) => {
        let transaction = db.transaction(['history'], 'readwrite');
        let historyStore = transaction.objectStore('history');

        // Check if an item for this URI exists on this day
        let currentDate = new Date();
        let year = currentDate.getFullYear();
        let month = currentDate.getMonth();
        let date = currentDate.getDate();
        let todayDate = new Date(year, month, date);

        let existingItemsIndex = historyStore.index('stampedURI');
        let lowerBound = [item.uri, todayDate];
        let upperBound = [item.uri, currentDate];
        let range = IDBKeyRange.bound(lowerBound, upperBound);
        let request = existingItemsIndex.openCursor(range);

        request.onsuccess = function(event) {
            let cursor = event.target.result;
            if (cursor) {
                // There's an entry for this URI, update the item
                cursor.value.timestamp = item.timestamp;
                let updateRequest = cursor.update(cursor.value);

                updateRequest.onsuccess = function(event) {
                    if (callback) {
                        callback(event.target.result.primaryKey);
                    }
                };
            } else {
                // No entry for this URI, add item
                let addItemRequest = historyStore.add(item);

                addItemRequest.onsuccess = function(event) {
                    if (callback) {
                        callback(event.target.result);
                    }
                };
            }
        };

    });
}

function updateHistoryItem(id, item, callback) {
    if (!id) {
        return;
    }

    queryDB((db) => {
        let transaction = db.transaction(['history'], 'readwrite');
        let historyStore = transaction.objectStore('history');
        let storedItemRequest = historyStore.get(id);
        storedItemRequest.onsuccess = function(event) {
            let storedItem = event.target.result;
            item.timestamp = storedItem.timestamp;

            let updateRequest = historyStore.put(item, id);

            updateRequest.onsuccess = function(event) {
                if (callback) {
                    callback();
                }
            };
        };

    });
}

function getHistoryItems(from, n, callback) {
    if (n <= 0 || from < 0) {
        if (callback) {
            callback([]);
        }
    }

    queryDB((db) => {
        let transaction = db.transaction(['history']);
        let historyStore = transaction.objectStore('history');
        let timestampIndex = historyStore.index('timestamp');
        let cursorRequest = timestampIndex.openCursor(null, 'prev');

        let current = 0;
        let items = [];
        cursorRequest.onsuccess = function(event) {
            let cursor = event.target.result;

            if (!cursor || current >= from + n) {
                if (callback) {
                    callback(items);
                }

                return;
            }

            if (current >= from) {
                items.push({
                    id: cursor.primaryKey,
                    item: cursor.value
                });
            }

            ++current;
            cursor.continue();
        };
    });
}

function removeHistoryItem(id, callback) {
    queryDB((db) => {
        let transaction = db.transaction(['history'], 'readwrite');
        let historyStore = transaction.objectStore('history');
        let removeItemRequest = historyStore.delete(id);

        removeItemRequest.onerror = function(event) {
            console.log(`Could not remove history item with ID: ${id}`);
            console.log(event.target.error.message);
        };

        removeItemRequest.onsuccess = function(event) {
            if (callback) {
                callback();
            }
        };
    });
}

function clearHistory(callback) {
    queryDB((db) => {
        let transaction = db.transaction(['history'], 'readwrite');
        let historyStore = transaction.objectStore('history');
        let clearRequest = historyStore.clear();

        clearRequest.onsuccess = function(event) {
            if (callback) {
                callback();
            }
        };
    });
}
