function isFavorite(uri, callback) {
    queryDB((db) => {
        let transaction = db.transaction(['favorites']);
        let favoritesStore = transaction.objectStore('favorites');
        let favoriteStatusRequest = favoritesStore.get(uri);

        favoriteStatusRequest.onerror = function(event) {
            console.log(`Could not query for ${uri}: ${event.target.error.message}`);
        };

        favoriteStatusRequest.onsuccess = function() {
            callback(favoriteStatusRequest.result);
        };
    });
}

function addFavorite(favorite, callback) {
    queryDB((db) => {
        let transaction = db.transaction(['favorites'], 'readwrite');
        let favoritesStore = transaction.objectStore('favorites');
        let addFavoriteRequest = favoritesStore.add(favorite);

        addFavoriteRequest.onerror = function(event) {
            console.log(`Could not add favorite with key: ${favorite.uri}`);
            console.log(event.target.error.message);
        };

        addFavoriteRequest.onsuccess = function(event) {
            if (callback) {
                callback();
            }
        };
    });
}

function removeFavorite(key, callback) {
    queryDB((db) => {
        let transaction = db.transaction(['favorites'], 'readwrite');
        let favoritesStore = transaction.objectStore('favorites');
        let removeFavoriteRequest = favoritesStore.delete(key);

        removeFavoriteRequest.onerror = function(event) {
            console.log(`Could not remove favorite with key: ${key}`);
            console.log(event.target.error.message);
        };

        removeFavoriteRequest.onsuccess = function(event) {
            if (callback) {
                callback();
            }
        };
    });
}

function getFavoritesAsJson(callback) {
    queryDB((db) => {
        let transaction = db.transaction(['favorites']);
        let favoritesStore = transaction.objectStore('favorites');
        let getFavoritesRequest = favoritesStore.getAll();

        getFavoritesRequest.onerror = function(event) {
            console.log(`Could retrieve favorites`);
            console.log(event.target.error.message);
        };

        getFavoritesRequest.onsuccess = function(event) {
            if (callback) {
                callback(getFavoritesRequest.result);
            }
        };
    });
}
