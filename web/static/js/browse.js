let offset = 0;
const limit = 20;
let loading = false;

// Fonction séparée pour charger l'image
function loadProfileImage(profileData, imageContainer) {
    // Photo par défaut si pas de photo de profil
    let photoUrl = '/static/images/default-profile.jpg';
    
    // Vérification et gestion améliorée des photos
    if (profileData.Profile.Photos && Array.isArray(profileData.Profile.Photos) && profileData.Profile.Photos.length > 0) {
        // Chercher la photo de profil
        const profilePhoto = profileData.Profile.Photos.find(p => p && p.IsProfile === true);
        
        if (profilePhoto && profilePhoto.FilePath) {
            // Debug log
            console.log('Photo trouvée:', profilePhoto);
            
            // Vérifier si le chemin commence par un slash
            photoUrl = profilePhoto.FilePath.startsWith('/') 
                ? profilePhoto.FilePath 
                : '/' + profilePhoto.FilePath;
        }
    }
    
    // Créer un élément image
    const img = new Image();
    
    // Gestionnaires d'événements
    img.onload = function() {
        // L'image est chargée, retirer la classe de chargement
        imageContainer.classList.remove('loading');
        imageContainer.appendChild(img);
    };
    
    img.onerror = function() {
        // En cas d'erreur, utiliser l'image par défaut
        console.warn('Image non trouvée, utilisation de l\'image par défaut:', photoUrl);
        img.src = '/static/images/default-profile.jpg';
        imageContainer.classList.remove('loading');
        imageContainer.appendChild(img);
    };
    
    // Ajouter des attributs à l'image
    img.src = photoUrl;
    img.alt = profileData.User.username || 'Photo de profil';
}

// Calcul de l'âge à partir d'une date de naissance
function calculateAge(birthDate) {
    const today = new Date();
    let age = today.getFullYear() - birthDate.getFullYear();
    const monthDiff = today.getMonth() - birthDate.getMonth();
    
    if (monthDiff < 0 || (monthDiff === 0 && today.getDate() < birthDate.getDate())) {
        age--;
    }
    
    return isNaN(age) ? '?' : age;
}

document.addEventListener('DOMContentLoaded', function() {
    // Charger les suggestions initiales
    loadSuggestions();

    // Gérer le formulaire de recherche
    const searchForm = document.getElementById('search-form');
    if (searchForm) {
        searchForm.addEventListener('submit', handleSearch);
    }

    // Gérer le bouton "Charger plus"
    const loadMoreBtn = document.getElementById('load-more');
    if (loadMoreBtn) {
        loadMoreBtn.addEventListener('click', loadMore);
    }
});

async function loadSuggestions() {
    if (loading) return;
    loading = true;

    const container = document.getElementById('profiles-container');
    
    // Créer ou récupérer le div de chargement
    let loadingDiv = document.querySelector('.loading');
    if (!loadingDiv) {
        loadingDiv = document.createElement('div');
        loadingDiv.className = 'loading';
        loadingDiv.textContent = 'Chargement des profils...';
        container.appendChild(loadingDiv);
    }

    try {
        const response = await fetch(`/api/suggestions?limit=${limit}&offset=${offset}`);
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const profiles = await response.json();
        console.log('Profiles received:', profiles);

        // Retirer le message de chargement
        if (loadingDiv && loadingDiv.parentNode) {
            loadingDiv.parentNode.removeChild(loadingDiv);
        }

        if (profiles.length === 0 && offset === 0) {
            container.innerHTML = '<p>Aucun profil trouvé. Réessayez plus tard.</p>';
            return;
        }

        // Afficher les profils
        profiles.forEach(profile => {
            const card = createProfileCard(profile);
            if (card) {
                container.appendChild(card);
            }
        });

        // Mise à jour de l'offset
        offset += profiles.length;

        // Masquer le bouton si plus de profils
        const loadMoreBtn = document.getElementById('load-more');
        if (loadMoreBtn) {
            loadMoreBtn.style.display = profiles.length < limit ? 'none' : 'block';
        }
    } catch (error) {
        console.error('Erreur:', error);
        if (loadingDiv && loadingDiv.parentNode) {
            loadingDiv.parentNode.removeChild(loadingDiv);
        }
        container.innerHTML = '<p>Erreur lors du chargement des profils. Veuillez réessayer.</p>';
    } finally {
        loading = false;
    }
}

async function handleSearch(e) {
    e.preventDefault();
    
    // Réinitialiser
    offset = 0;
    const container = document.getElementById('profiles-container');
    container.innerHTML = '';
    
    // Créer les paramètres de recherche
    const params = new URLSearchParams();
    
    // Filtrage par âge
    const minAge = document.getElementById('min_age').value;
    if (minAge) params.append('min_age', minAge);
    
    const maxAge = document.getElementById('max_age').value;
    if (maxAge) params.append('max_age', maxAge);
    
    // Filtrage par fame rating
    const minFame = document.getElementById('min_fame').value;
    if (minFame) params.append('min_fame', minFame);
    
    const maxFame = document.getElementById('max_fame').value;
    if (maxFame) params.append('max_fame', maxFame);
    
    const maxDistance = document.getElementById('max_distance').value;
    if (maxDistance) params.append('max_distance', maxDistance);
    
    const tags = document.getElementById('tags').value;
    if (tags) params.append('tags', tags);
    
    const sortBy = document.getElementById('sort_by').value;
    if (sortBy) params.append('sort_by', sortBy);
    
    const sortOrder = document.getElementById('sort_order').value;
    if (sortOrder) params.append('sort_order', sortOrder);
    
    params.append('limit', limit);
    params.append('offset', offset);

    // Marquer que nous sommes en mode recherche
    const searchForm = document.getElementById('search-form');
    searchForm.dataset.searching = 'true';

    try {
        const response = await fetch(`/api/search?${params.toString()}`);
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const profiles = await response.json();
        console.log('Search results:', profiles);
        
        if (profiles.length === 0) {
            container.innerHTML = '<p>Aucun profil ne correspond à vos critères</p>';
            return;
        }

        // Afficher les profils
        profiles.forEach(profile => {
            const card = createProfileCard(profile);
            if (card) {
                container.appendChild(card);
            }
        });

        // Mise à jour de l'offset
        offset += profiles.length;

        // Afficher le bouton "Charger plus"
        const loadMoreBtn = document.getElementById('load-more');
        if (loadMoreBtn) {
            loadMoreBtn.style.display = profiles.length < limit ? 'none' : 'block';
        }
    } catch (error) {
        console.error('Erreur:', error);
        container.innerHTML = '<p>Erreur lors de la recherche. Veuillez réessayer.</p>';
    }
}

// Fonction mise à jour pour createProfileCard dans browse.js
function createProfileCard(profileData) {
    // Vérifier que nous avons les données nécessaires
    if (!profileData || !profileData.Profile || !profileData.User) {
        console.error('Données de profil invalides:', profileData);
        
        // Créer une carte d'erreur
        const errorCard = document.createElement('div');
        errorCard.className = 'profile-card error';
        return errorCard;
    }

    const card = document.createElement('div');
    card.className = 'profile-card';
    
    // Créer le conteneur d'image en premier
    const imageContainer = document.createElement('div');
    imageContainer.className = 'profile-card-image loading';
    
    // Créer le contenu initial de la carte (sans l'image)
    const contentDiv = document.createElement('div');
    contentDiv.className = 'profile-card-content';
    
    // Calculer l'âge à partir de la date de naissance
    let age = '?';
    if (profileData.Profile.BirthDate) {
        try {
            age = calculateAge(new Date(profileData.Profile.BirthDate));
        } catch (e) {
            console.error('Erreur lors du calcul de l\'âge:', e);
        }
    }
    
    // Sécuriser les valeurs pour éviter les erreurs
    const username = profileData.User.username || 'Inconnu';
    const distance = profileData.Distance ? profileData.Distance.toFixed(1) : '0';
    const fameRating = profileData.Profile.FameRating || 0;
    const compatibilityScore = profileData.CompatibilityScore ? (profileData.CompatibilityScore * 100).toFixed(0) : '0';
    
    // Gérer les tags
    let tagsHtml = '';
    if (profileData.Profile.Tags && Array.isArray(profileData.Profile.Tags)) {
        tagsHtml = profileData.Profile.Tags.slice(0, 3).map(tag => {
            const tagName = tag && (tag.Name || tag.name) ? (tag.Name || tag.name) : '';
            return tagName ? `<span class="tag">${tagName}</span>` : '';
        }).join('');
        
        if (profileData.Profile.Tags.length > 3) {
            tagsHtml += '<span class="tag">...</span>';
        }
    }
    
    // Définir le contenu HTML
    contentDiv.innerHTML = `
        <h3>${username}, ${age}</h3>
        <p>Distance: ${distance} km</p>
        <p>Fame: ${fameRating}/100</p>
        <p>Compatibilité: ${compatibilityScore}%</p>
        <div class="profile-tags">
            ${tagsHtml}
        </div>
        <div class="profile-actions">
            <button class="view-button" onclick="viewProfile(${profileData.User.id})">Voir le profil</button>
            <button class="like-button" onclick="likeProfile(${profileData.User.id})">♥ Like</button>
        </div>
    `;
    
    // Ajouter le conteneur d'image et le contenu à la carte
    card.appendChild(imageContainer);
    card.appendChild(contentDiv);
    
    // Charger l'image en parallèle
    loadProfileImage(profileData, imageContainer);
    
    return card;
}

async function viewProfile(userId) {
    window.location.href = `/profile/${userId}`;
}

async function likeProfile(userId) {
    try {
        const response = await fetch(`/api/profile/${userId}/like`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            }
        });

        if (response.ok) {
            const data = await response.json();
            if (data.matched) {
                alert('C\'est un match ! Vous pouvez maintenant discuter.');
            } else {
                alert('Like envoyé !');
            }
        } else {
            const data = await response.json();
            alert(data.message || 'Erreur lors du like');
        }
    } catch (error) {
        console.error('Erreur:', error);
        alert('Erreur lors du like');
    }
}

function loadMore() {
    const searchForm = document.getElementById('search-form');
    if (searchForm && searchForm.dataset.searching === 'true') {
        handleSearch(new Event('submit'));
    } else {
        loadSuggestions();
    }
}