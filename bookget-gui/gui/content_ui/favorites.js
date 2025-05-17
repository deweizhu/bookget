const messageHandler = event => {
    var message = event.data.message;
    var args = event.data.args;

    switch (message) {
        case commands.MG_GET_FAVORITES:
            loadFavorites(args.favorites);
            break;
        default:
            console.log(`Unexpected message: ${JSON.stringify(event.data)}`);
            break;
    }
};

function requestFavorites() {
    let message = {
        message: commands.MG_GET_FAVORITES,
        args: {}
    };

    window.chrome.webview.postMessage(message);
}

function removeFavorite(uri) {
    let message = {
        message: commands.MG_REMOVE_FAVORITE,
        args: {
            uri: uri
        }
    };

    window.chrome.webview.postMessage(message);
}

function loadFavorites(payload) {
    let fragment = document.createDocumentFragment();

    if (payload.length > 0) {
        let container = document.getElementById('entries-container');
        container.textContent = '';
    }

    payload.map(favorite => {
        let favoriteContainer = document.createElement('div');
        favoriteContainer.className = 'item-container';
        let favoriteElement = document.createElement('div');
        favoriteElement.className = 'item';

        let faviconElement = document.createElement('div');
        faviconElement.className = 'favicon';
        let faviconImage = document.createElement('img');
        faviconImage.src = favorite.favicon;
        faviconElement.appendChild(faviconImage);

        let labelElement = document.createElement('div');
        labelElement.className = 'label-title';
        let linkElement = document.createElement('a');
        linkElement.textContent = favorite.title;
        linkElement.href = favorite.uri;
        linkElement.title = favorite.title;
        labelElement.appendChild(linkElement);

        let uriElement = document.createElement('div');
        uriElement.className = 'label-uri';
        let textElement = document.createElement('p');
        textElement.textContent = favorite.uriToShow || favorite.uri;
        textElement.title = favorite.uriToShow || favorite.uri;
        uriElement.appendChild(textElement);

        let buttonElement = document.createElement('div');
        buttonElement.className = 'btn-close';
        buttonElement.addEventListener('click', function(e) {
            favoriteContainer.parentNode.removeChild(favoriteContainer);
            removeFavorite(favorite.uri);
        });

        favoriteElement.appendChild(faviconElement);
        favoriteElement.appendChild(labelElement);
        favoriteElement.appendChild(uriElement);
        favoriteElement.appendChild(buttonElement);

        favoriteContainer.appendChild(favoriteElement);
        fragment.appendChild(favoriteContainer);
    });

    let container = document.getElementById('entries-container');
    container.appendChild(fragment);
}

function init() {
    window.chrome.webview.addEventListener('message', messageHandler);
    requestFavorites();
}

init();
