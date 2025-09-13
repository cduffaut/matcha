// browse.js - Gestion de la page d'exploration des profils

document.addEventListener('DOMContentLoaded', function() {
    let currentOffset = 0;
    const limit = 20;
    let isLoading = false;
    let hasMoreResults = true;
    let selectedTags = [];
    let availableTags = [];

    // Éléments DOM
    const profilesContainer = document.getElementById('profiles-container');
    const searchForm = document.getElementById('search-form');
    const loadMoreBtn = document.getElementById('load-more');

    // Initialisation
    init();

    function init() {
        setupEventListeners();
        loadSuggestions();
        injectAdditionalCSS();
        initTagsSearch();
    }

    // Configuration des écouteurs d'événements
    function setupEventListeners() {
        // Formulaire de recherche
        if (searchForm) {
            searchForm.addEventListener('submit', handleSearchSubmit);
        }

        // Bouton charger plus
        if (loadMoreBtn) {
            loadMoreBtn.addEventListener('click', loadMoreProfiles);
        }
    }

    // Gestion de la soumission du formulaire de recherche
    async function handleSearchSubmit(e) {
        e.preventDefault();
        
        if (isLoading) return;
        
        const formData = new FormData(searchForm);
        const searchParams = {};
        
        // Construire les paramètres de recherche
        for (let [key, value] of formData.entries()) {
            if (value.trim() !== '') {
                searchParams[key] = value.trim();
            }
        }

        if (selectedTags.length > 0) {
            // Enlever les # pour l'envoi au backend (plus simple à traiter côté serveur)
            const cleanTags = selectedTags.map(tag => tag.startsWith('#') ? tag.substring(1) : tag);
            searchParams['tags'] = cleanTags.join(',');
        }
        
        // Réinitialiser l'offset pour une nouvelle recherche
        currentOffset = 0;
        hasMoreResults = true;
        
        await handleSearch(searchParams);
    }

    // Fonction principale pour charger les suggestions
    async function loadSuggestions() {
        if (isLoading) return;
        
        isLoading = true;
        showLoading();

        try {
            const response = await fetch(`/api/suggestions?limit=${limit}&offset=${currentOffset}`);
            
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            
            const data = await response.json();
            
            // Gérer le cas du profil incomplet
            if (data.profile_incomplete) {
                displayProfileIncompleteMessage(data.message);
                hideLoadMoreButton();
                return;
            }
            
            const suggestions = data.suggestions || [];
            
            if (currentOffset === 0) {
                // Première charge - remplacer le contenu
                displayProfiles(suggestions);
            } else {
                // Charges suivantes - ajouter au contenu existant
                appendProfiles(suggestions);
            }
            
            // Gérer la pagination
            if (suggestions.length < limit) {
                hasMoreResults = false;
                hideLoadMoreButton();
            } else {
                showLoadMoreButton();
            }
            
        } catch (error) {
            displayError('Erreur lors du chargement des profils. Veuillez réessayer.');
        } finally {
            isLoading = false;
            hideLoading();
        }
    }

    // Fonction pour gérer la recherche
    async function handleSearch(searchParams) {
        if (isLoading) return;
        
        isLoading = true;
        showLoading();

        try {
            const params = new URLSearchParams({
                ...searchParams,
                limit: limit,
                offset: currentOffset
            });
            
            const response = await fetch(`/api/search?${params.toString()}`);
            
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            
            const data = await response.json();
            
            // Gérer le cas du profil incomplet
            if (data.profile_incomplete) {
                displayProfileIncompleteMessage(data.message);
                hideLoadMoreButton();
                return;
            }
            
            const results = data.results || [];
            
            if (currentOffset === 0) {
                displayProfiles(results);
            } else {
                appendProfiles(results);
            }
            
            // Gérer la pagination
            if (results.length < limit) {
                hasMoreResults = false;
                hideLoadMoreButton();
            } else {
                showLoadMoreButton();
            }
            
        } catch (error) {
            displayError('Erreur lors de la recherche. Veuillez réessayer.');
        } finally {
            isLoading = false;
            hideLoading();
        }
    }

    // Charger plus de profils
    async function loadMoreProfiles() {
        if (!hasMoreResults || isLoading) return;
        
        currentOffset += limit;
        await loadSuggestions();
    }

    // Afficher les profils (remplace le contenu)
    function displayProfiles(profiles) {
        if (!profilesContainer) return;
        
        if (profiles.length === 0) {
            profilesContainer.innerHTML = `
                <div class="no-profiles">
                    <h3>Aucun profil trouvé</h3>
                    <p>Aucun profil ne correspond à vos critères de recherche.</p>
                    <button onclick="location.reload()" class="retry-btn">Voir toutes les suggestions</button>
                </div>
            `;
            hideLoadMoreButton();
            return;
        }
        
        profilesContainer.innerHTML = profiles.map(createProfileCard).join('');
        currentOffset = profiles.length;
    }

    // Ajouter des profils au contenu existant
    function appendProfiles(profiles) {
        if (!profilesContainer || profiles.length === 0) return;
        
        const profilesHTML = profiles.map(createProfileCard).join('');
        profilesContainer.insertAdjacentHTML('beforeend', profilesHTML);
        currentOffset += profiles.length;
    }

    // Créer une carte de profil
    function createProfileCard(profile) {
        const user = profile.User || {};
        const profileData = profile.Profile || {};
        
        // Calculer l'âge
        let ageText = 'Âge non spécifié';
        if (profileData.BirthDate || profileData.birth_date) {
            const birthDateStr = profileData.BirthDate || profileData.birth_date;
            const birthDate = new Date(birthDateStr);
            const today = new Date();
            let age = today.getFullYear() - birthDate.getFullYear();
            const monthDiff = today.getMonth() - birthDate.getMonth();
            if (monthDiff < 0 || (monthDiff === 0 && today.getDate() < birthDate.getDate())) {
                age--;
            }
            ageText = `${age} ans`;
        }
        
        // Photo de profil
        let profilePhotoUrl = '/static/images/default-profile.jpg';
        const photos = profileData.Photos || profileData.photos;
        if (photos && photos.length > 0) {
            const profilePhoto = photos.find(photo => photo.IsProfile || photo.is_profile);
            if (profilePhoto) {
                profilePhotoUrl = profilePhoto.FilePath || profilePhoto.file_path;
            }
        }
        
        // Tags
        const tags = profileData.Tags || profileData.tags;
        const tagsHTML = tags && tags.length > 0 
            ? tags.map(tag => `<span class="tag">${tag.Name || tag.name}</span>`).join('')
            : '<span class="no-tags">Aucun intérêt</span>';
        
        // Distance
        const distanceText = profile.Distance 
            ? `${Math.round(profile.Distance)} km` 
            : 'Distance inconnue';
        
        // Score de compatibilité
        const compatibilityScore = profile.CompatibilityScore 
            ? Math.round(profile.CompatibilityScore * 100) 
            : 0;

        // Utiliser les clés avec majuscules qui correspondent à l'API
        const userId = user.ID || user.id;
        const firstName = user.FirstName || user.first_name;
        const lastName = user.LastName || user.last_name;
        const biography = profileData.Biography || profileData.biography;
        const fameRating = profileData.FameRating || profileData.fame_rating;

        return `
            <div class="profile-card" data-user-id="${userId}">
                <div class="profile-photo">
                    <img src="${profilePhotoUrl}" alt="Photo de ${firstName}" loading="lazy">
                    <div class="compatibility-score">${compatibilityScore}%</div>
                </div>
                <div class="profile-info">
                    <h3>${firstName} ${lastName}</h3>
                    <p class="profile-details">
                        <span class="age">${ageText}</span> • 
                        <span class="distance">${distanceText}</span> • 
                        <span class="fame">Fame: ${fameRating || 0}</span>
                    </p>
                    <div class="profile-bio">
                        <p>${biography || 'Aucune biographie'}</p>
                    </div>
                    <div class="profile-tags">
                        ${tagsHTML}
                    </div>
                    <div class="profile-actions">
                        <button onclick="viewProfile(${userId})" class="view-btn">Voir le profil</button>
                        <button onclick="likeProfile(${userId})" class="like-btn">👍 Liker</button>
                    </div>
                </div>
            </div>
        `;
    }

    // Afficher un message de profil incomplet
    function displayProfileIncompleteMessage(message) {
        if (!profilesContainer) return;
        
        profilesContainer.innerHTML = `
            <div class="profile-incomplete-message">
                <div class="message-icon">⚠️</div>
                <h3>Profil incomplet</h3>
                <p>${message || 'Complétez votre profil pour voir des suggestions'}</p>
                <p>Pour voir des suggestions et interagir avec d'autres utilisateurs, vous devez compléter votre profil avec :</p>
                <ul>
                    <li>✅ Genre</li>
                    <li>✅ Préférences sexuelles</li>
                    <li>✅ Biographie</li>
                    <li>✅ Date de naissance</li>
                    <li>✅ Au moins un intérêt (tag)</li>
                    <li>✅ Au moins une photo de profil</li>
                </ul>
                <a href="/profile" class="complete-profile-btn">Compléter mon profil</a>
            </div>
        `;
    }

    // Afficher une erreur générale
    function displayError(message) {
        if (!profilesContainer) return;
        
        profilesContainer.innerHTML = `
            <div class="error-message">
                <div class="message-icon">❌</div>
                <h3>Erreur</h3>
                <p>${message}</p>
                <button onclick="location.reload()" class="retry-btn">Réessayer</button>
            </div>
        `;
    }

    // Afficher l'indicateur de chargement
    function showLoading() {
        if (!profilesContainer) return;
        
        if (currentOffset === 0) {
            // Premier chargement
            profilesContainer.innerHTML = `
                <div class="loading">
                    <div class="loading-spinner"></div>
                    <p>Chargement des profils...</p>
                </div>
            `;
        } else {
            // Chargement de plus de résultats
            if (loadMoreBtn) {
                loadMoreBtn.textContent = 'Chargement...';
                loadMoreBtn.disabled = true;
            }
        }
    }

    // Masquer l'indicateur de chargement
    function hideLoading() {
        if (loadMoreBtn) {
            loadMoreBtn.textContent = 'Charger plus';
            loadMoreBtn.disabled = false;
        }
    }

    // Afficher le bouton "Charger plus"
    function showLoadMoreButton() {
        if (loadMoreBtn) {
            loadMoreBtn.style.display = 'block';
        }
    }

    // Masquer le bouton "Charger plus"
    function hideLoadMoreButton() {
        if (loadMoreBtn) {
            loadMoreBtn.style.display = 'none';
        }
    }

    // Injecter le CSS supplémentaire
    function injectAdditionalCSS() {
        const additionalCSS = `
            .profile-incomplete-message,
            .error-message,
            .no-profiles {
                text-align: center;
                padding: 2rem;
                background: #f8f9fa;
                border-radius: 8px;
                margin: 2rem 0;
                border: 1px solid #dee2e6;
            }

            .message-icon {
                font-size: 3rem;
                margin-bottom: 1rem;
            }

            .profile-incomplete-message h3,
            .error-message h3,
            .no-profiles h3 {
                color: #495057;
                margin-bottom: 1rem;
            }

            .profile-incomplete-message ul {
                text-align: left;
                display: inline-block;
                margin: 1rem 0;
            }

            .profile-incomplete-message li {
                margin: 0.5rem 0;
                padding-left: 0.5rem;
            }

            .complete-profile-btn,
            .retry-btn {
                display: inline-block;
                background-color: #4CAF50;
                color: white;
                padding: 0.8rem 1.5rem;
                text-decoration: none;
                border-radius: 4px;
                font-weight: bold;
                margin-top: 1rem;
                border: none;
                cursor: pointer;
                transition: background-color 0.3s;
            }

            .complete-profile-btn:hover,
            .retry-btn:hover {
                background-color: #45a049;
            }

            .loading {
                text-align: center;
                padding: 2rem;
            }

            .loading-spinner {
                border: 4px solid #f3f3f3;
                border-top: 4px solid #4CAF50;
                border-radius: 50%;
                width: 40px;
                height: 40px;
                animation: spin 1s linear infinite;
                margin: 0 auto 1rem;
            }

            @keyframes spin {
                0% { transform: rotate(0deg); }
                100% { transform: rotate(360deg); }
            }

            .profile-card {
                border: 1px solid #ddd;
                border-radius: 8px;
                padding: 1rem;
                margin: 1rem 0;
                background: white;
                box-shadow: 0 2px 4px rgba(0,0,0,0.1);
                transition: transform 0.2s, box-shadow 0.2s;
            }

            .profile-card:hover {
                transform: translateY(-2px);
                box-shadow: 0 4px 8px rgba(0,0,0,0.15);
            }

            .profile-photo {
                position: relative;
                text-align: center;
                margin-bottom: 1rem;
            }

            .profile-photo img {
                width: 150px;
                height: 150px;
                border-radius: 50%;
                object-fit: cover;
                border: 3px solid #4CAF50;
            }

            .compatibility-score {
                position: absolute;
                top: 10px;
                right: 10px;
                background: #4CAF50;
                color: white;
                padding: 4px 8px;
                border-radius: 12px;
                font-size: 0.8rem;
                font-weight: bold;
            }

            .profile-info h3 {
                margin: 0 0 0.5rem 0;
                color: #333;
            }

            .profile-details {
                color: #666;
                font-size: 0.9rem;
                margin-bottom: 1rem;
            }

            .profile-bio p {
                color: #555;
                font-style: italic;
                margin-bottom: 1rem;
            }

            .profile-tags {
                margin-bottom: 1rem;
            }

            .tag {
                display: inline-block;
                background: #e9ecef;
                color: #495057;
                padding: 2px 8px;
                border-radius: 12px;
                font-size: 0.8rem;
                margin: 2px;
            }

            .no-tags {
                color: #999;
                font-style: italic;
            }

            .profile-actions {
                display: flex;
                gap: 0.5rem;
                justify-content: center;
            }

            .view-btn,
            .like-btn {
                padding: 0.5rem 1rem;
                border: none;
                border-radius: 4px;
                cursor: pointer;
                font-weight: bold;
                transition: background-color 0.3s;
            }

            .view-btn {
                background: #007bff;
                color: white;
            }

            .view-btn:hover {
                background: #0056b3;
            }

            .like-btn {
                background: #4CAF50;
                color: white;
            }

            .like-btn:hover {
                background: #45a049;
            }

            .tags-search-group {
                position: relative;
                width: 100%;
                display: flex;
                flex-direction: column;
                gap: 0.5rem;
            }

            .tags-input-row {
                display: flex;
                gap: 0.5rem;
                align-items: stretch;
            }

            #tags-search {
                flex: 1;
                margin-bottom: 0 !important;
            }

            #selected-tags {
                display: flex;
                flex-wrap: wrap;
                gap: 0.5rem;
                margin: 0.5rem 0;
                min-height: 32px; /* Force une hauteur minimale */
                padding: 0.5rem;
                border: 1px dashed #ddd;
                border-radius: 4px;
                background-color: #fafafa;
            }

            #selected-tags:empty::before {
                content: "Aucun tag sélectionné";
                color: #999;
                font-style: italic;
            }

            .selected-tag {
                background-color: #4CAF50 !important;
                color: white !important;
                padding: 0.4rem 0.8rem;
                border-radius: 20px;
                display: inline-flex;
                align-items: center;
                gap: 0.5rem;
                font-size: 0.85rem;
                font-weight: 500;
            }

            .selected-tag .remove-tag {
                background: none;
                border: none;
                color: white;
                cursor: pointer;
                padding: 0;
                font-size: 1.1rem;
                font-weight: bold;
                width: 16px;
                height: 16px;
                border-radius: 50%;
                display: flex;
                align-items: center;
                justify-content: center;
            }

            .selected-tag .remove-tag:hover {
                background-color: rgba(255,255,255,0.2);
            }

            #add-tag-btn {
                padding: 0.4rem 0.8rem !important;
                font-size: 0.9rem !important;
                height: auto !important;
                align-self: stretch;
                white-space: nowrap;
            }

            #tag-suggestions {
                position: absolute;
                top: 100%;
                left: 0;
                right: 0;
                background: white;
                border: 1px solid #ddd;
                border-radius: 4px;
                max-height: 200px;
                overflow-y: auto;
                z-index: 1000;
                display: none;
                box-shadow: 0 2px 8px rgba(0,0,0,0.15);
                margin-top: 2px;
            }

            .tag-suggestion {
                padding: 0.6rem;
                cursor: pointer;
                border-bottom: 1px solid #f0f0f0;
                transition: background-color 0.2s;
            }

            .tag-suggestion:hover {
                background-color: #f0f0f0;
            }

            .tag-suggestion:last-child {
                border-bottom: none;
            }

            .search-summary {
                background-color: #e8f5e8;
                border: 1px solid #4CAF50;
                border-radius: 4px;
                padding: 1rem;
                margin-top: 1rem;
            }

            .search-summary h4 {
                margin: 0 0 0.5rem 0;
                color: #2e7d32;
            }

            .search-summary ul {
                margin: 0;
                padding-left: 1.5rem;
            }

            .search-summary li {
                color: #2e7d32;
                margin-bottom: 0.25rem;
            }

            #load-more {
                display: block;
                margin: 2rem auto;
                padding: 1rem 2rem;
                background: #4CAF50;
                color: white;
                border: none;
                border-radius: 4px;
                cursor: pointer;
                font-weight: bold;
                transition: background-color 0.3s;
            }

            #load-more:hover:not(:disabled) {
                background: #45a049;
            }

            #load-more:disabled {
                background: #ccc;
                cursor: not-allowed;
            }
        `;

        const styleSheet = document.createElement("style");
        styleSheet.textContent = additionalCSS;
        document.head.appendChild(styleSheet);
    }

    // Fonctions globales pour les interactions
    window.viewProfile = function(userId) {
        window.location.href = `/profile/${userId}`;
    };

    window.likeProfile = async function(userId) {
        try {
            const response = await fetch(`/api/profile/${userId}/like`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                }
            });

            const data = await response.json();

            if (response.ok) {
                if (data.matched) {
                    alert('🎉 C\'est un match ! Vous pouvez maintenant discuter ensemble.');
                } else {
                    alert('👍 Like envoyé !');
                }
                // Recharger les suggestions pour mettre à jour l'affichage
                currentOffset = 0;
                loadSuggestions();
            } else {
                alert(data.error || 'Erreur lors du like');
            }
        } catch (error) {
            alert('Erreur lors du like');
        }
    };

    // Initialiser le système de tags
    function initTagsSearch() {
        const tagInput = document.getElementById('tags-search');
        const addTagBtn = document.getElementById('add-tag-btn');
        const suggestionsContainer = document.getElementById('tag-suggestions');
        
        if (!tagInput) return;
        
        // Charger tous les tags disponibles
        loadAvailableTags();
        
        // Event listeners
        tagInput.addEventListener('input', handleTagInput);
        tagInput.addEventListener('keypress', handleTagKeypress);
        addTagBtn.addEventListener('click', addCurrentTag);
        
        // Tags populaires
        document.querySelectorAll('.popular-tag').forEach(tag => {
            tag.addEventListener('click', () => {
                const tagName = tag.dataset.tag;
                addTag(tagName);
            });
        });
        
        // Cacher les suggestions quand on clique ailleurs
        document.addEventListener('click', (e) => {
            if (!e.target.closest('.tags-search-group')) {
                hideSuggestions();
            }
        });
    }

    // Charger tous les tags disponibles depuis l'API
    async function loadAvailableTags() {
        try {
            const response = await fetch('/api/tags');
            if (response.ok) {
                const tags = await response.json();
                availableTags = tags.map(tag => {
                    const tagName = tag.name || tag.Name;
                    // S'assurer que tous les tags ont un #
                    return tagName.startsWith('#') ? tagName : '#' + tagName;
                });
            }
        } catch (error) {
            // Tags par défaut avec #
            availableTags = ['#sport', '#cuisine', '#voyage', '#cinéma', '#musique', '#lecture', '#art', '#technologie'];
        }
    }

    // Gérer la saisie dans le champ tag
    function handleTagInput(e) {
        let query = e.target.value.toLowerCase().trim();
        
        // Enlever le # du début pour la recherche si présent
        if (query.startsWith('#')) {
            query = query.substring(1);
        }
        
        if (query.length >= 2) {
            const suggestions = availableTags.filter(tag => {
                // Enlever le # du tag pour la comparaison
                const tagWithoutHash = tag.startsWith('#') ? tag.substring(1) : tag;
                const normalizedSelected = selectedTags.map(t => t.startsWith('#') ? t.substring(1) : t);
                
                return tagWithoutHash.toLowerCase().includes(query) && 
                    !normalizedSelected.includes(tagWithoutHash);
            });
            showSuggestions(suggestions);
        } else {
            hideSuggestions();
        }
    }

    // Gérer la touche Entrée
    function handleTagKeypress(e) {
        if (e.key === 'Enter') {
            e.preventDefault();
            addCurrentTag();
        }
    }

    // Ajouter le tag actuellement tapé
    function addCurrentTag() {
        const tagInput = document.getElementById('tags-search');
        if (!tagInput) {
            return;
        }
        
        const tagName = tagInput.value.trim();
        
        if (tagName) {
            addTag(tagName);
            tagInput.value = '';
            hideSuggestions();
        }
    }

    // Ajouter un tag à la sélection
    function addTag(tagName) {
        // Normaliser le tag : ajouter # si absent
        let normalizedTag = tagName.trim();
        if (!normalizedTag.startsWith('#')) {
            normalizedTag = '#' + normalizedTag;
        }
        
        // Éviter les doublons
        if (selectedTags.includes(normalizedTag)) {
            return;
        }
        
        selectedTags.push(normalizedTag);
        updateSelectedTagsDisplay();
        updateSearchSummary();

        const container = document.getElementById('selected-tags');
        if (container) {
            container.style.backgroundColor = '#e8f5e8';
            setTimeout(() => {
                container.style.backgroundColor = '#fafafa';
            }, 300);
        }
    }

    // Supprimer un tag de la sélection
    function removeTag(tagName) {
        selectedTags = selectedTags.filter(tag => tag !== tagName);
        updateSelectedTagsDisplay();
        updateSearchSummary();
    }

    // Mettre à jour l'affichage des tags sélectionnés
    function updateSelectedTagsDisplay() {
        const container = document.getElementById('selected-tags');
        if (!container) return;
        
        if (selectedTags.length === 0) {
            container.innerHTML = '';
            return;
        }
        
        // Vider le conteneur
        container.innerHTML = '';
        
        // Créer chaque tag individuellement pour éviter les problèmes d'échappement
        selectedTags.forEach(tag => {
            const tagElement = document.createElement('span');
            tagElement.className = 'selected-tag';
            
            const tagText = document.createElement('span');
            tagText.textContent = tag;
            
            const removeButton = document.createElement('button');
            removeButton.type = 'button';
            removeButton.className = 'remove-tag';
            removeButton.textContent = '×';
            removeButton.title = 'Supprimer';
            
            // Ajouter l'événement directement (pas en inline)
            removeButton.addEventListener('click', () => {
                removeTag(tag);
            });
            
            tagElement.appendChild(tagText);
            tagElement.appendChild(removeButton);
            container.appendChild(tagElement);
        });
    }

    // Afficher les suggestions
    function showSuggestions(suggestions) {
        const container = document.getElementById('tag-suggestions');
        if (!container) return;
        
        if (suggestions.length === 0) {
            hideSuggestions();
            return;
        }
        
		container.innerHTML = '';
		suggestions.forEach(tag => {
			const suggestionDiv = document.createElement('div');
			suggestionDiv.className = 'tag-suggestion';
			suggestionDiv.textContent = tag;
			suggestionDiv.addEventListener('click', () => {
			addTag(tag);
			const tagInput = document.getElementById('tags-search');
			if (tagInput) tagInput.value = '';
			hideSuggestions();
			});
			container.appendChild(suggestionDiv);
		});
        
        container.style.display = 'block';
    }

    // Cacher les suggestions
    function hideSuggestions() {
        const container = document.getElementById('tag-suggestions');
        if (container) {
            container.style.display = 'none';
        }
    }

    // Mettre à jour le résumé de recherche
    function updateSearchSummary() {
        // Créer ou mettre à jour un résumé des critères de recherche
        let summaryContainer = document.getElementById('search-summary');
        
        if (!summaryContainer) {
            // Créer le conteneur s'il n'existe pas
            summaryContainer = document.createElement('div');
            summaryContainer.id = 'search-summary';
            summaryContainer.className = 'search-summary';
            
            const searchForm = document.getElementById('search-form');
            if (searchForm) {
                searchForm.appendChild(summaryContainer);
            }
        }
        
        // Construire le résumé
        const criteria = [];
        
        if (selectedTags.length > 0) {
            criteria.push(`Tags: ${selectedTags.join(', ')}`);
        }
        
        // Autres critères (âge, distance, etc.)
        const ageMin = document.getElementById('age-min')?.value;
        const ageMax = document.getElementById('age-max')?.value;
        if (ageMin || ageMax) {
            criteria.push(`Âge: ${ageMin || '?'} - ${ageMax || '?'} ans`);
        }
        
        const maxDistance = document.getElementById('max-distance')?.value;
        if (maxDistance) {
            criteria.push(`Distance: max ${maxDistance} km`);
        }
        
        if (criteria.length > 0) {
            summaryContainer.innerHTML = `
                <h4>🔍 Critères de recherche actifs:</h4>
                <ul>
                    ${criteria.map(criterion => `<li>${criterion}</li>`).join('')}
                </ul>
            `;
            summaryContainer.style.display = 'block';
        } else {
            summaryContainer.style.display = 'none';
        }
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

});