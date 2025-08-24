// Gérer les mises à jour du profil
document.addEventListener('DOMContentLoaded', function() {
    // Géolocalisation automatique si pas de localisation
    setTimeout(initializeLocationIfMissing, 2000); // Délai de 2 secondes
    // Gérer la soumission du formulaire de profil
    const saveProfileBtn = document.getElementById('save-profile');
    if (saveProfileBtn) {
        saveProfileBtn.addEventListener('click', saveProfile);
    }

    // Gérer l'ajout de tags
    const addTagBtn = document.getElementById('add-tag-btn');
    if (addTagBtn) {
        addTagBtn.addEventListener('click', addTag);
    }
    
    // Gérer l'entrée pour ajouter un tag avec la touche Enter
    const newTagInput = document.getElementById('new-tag');
    if (newTagInput) {
        newTagInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                e.preventDefault();
                addTag(e);
            }
        });
    }

    // Gérer la suppression de tags
    document.addEventListener('click', function(e) {
        if (e.target.classList.contains('remove-tag')) {
            const tagId = e.target.dataset.id;
            removeTag(tagId);
        }
    });

    // Gérer l'upload de photos
    const photoForm = document.getElementById('photo-form');
    if (photoForm) {
        photoForm.addEventListener('submit', uploadPhoto);
    }

    // Gérer la définition de la photo de profil
    document.addEventListener('click', function(e) {
        if (e.target.classList.contains('set-profile-photo')) {
            const photoContainer = e.target.closest('.photo-container');
            const photoId = photoContainer.dataset.id;
            setProfilePhoto(photoId);
        }
    });

    document.addEventListener('click', function(e) {
        if (e.target.classList.contains('delete-photo')) {
            const photoContainer = e.target.closest('.photo-container');
            const photoId = photoContainer.dataset.id;
            deletePhoto(photoId);  
        }
    });

    // Gérer la mise à jour manuelle de la localisation
    const updateLocationBtn = document.getElementById('update-location');
    if (updateLocationBtn) {
        updateLocationBtn.addEventListener('click', updateLocation);
    }

    // Fonction pour permettre la modification manuelle (garder votre fonction existante)
    window.enableManualLocation = function() {
        const locationInput = document.getElementById('location');
        if (locationInput) {
            locationInput.readOnly = false;
            locationInput.placeholder = "Entrez votre ville ou coordonnées (ex: Paris ou 48.8566, 2.3522)";
        }
    };
});

// Fonction pour sauvegarder le profil
async function saveProfile(e) {
    e.preventDefault();

    // Récupérer et formater la date de naissance
    const birthDateInput = document.getElementById('birth_date').value;
    let birthDate = null;
    if (birthDateInput) {
        birthDate = birthDateInput + 'T00:00:00Z';
    }

    // Récupérer les valeurs des champs correctement
    const genderSelect = document.getElementById('gender');
    const sexualPrefSelect = document.getElementById('sexual_preference');
    const biographyTextarea = document.getElementById('biography');
    const locationInput = document.getElementById('location');

    const profileData = {
        gender: genderSelect ? genderSelect.value : '',
        sexual_preference: sexualPrefSelect ? sexualPrefSelect.value : '',
        biography: biographyTextarea ? biographyTextarea.value.trim() : '',
        birth_date: birthDate,
        location_name: locationInput ? locationInput.value.trim() : ''
    };

    // Gérer les coordonnées existantes
    if (profileData.location_name && profileData.location_name.includes(',')) {
        const coords = profileData.location_name.split(',');
        if (coords.length === 2) {
            const lat = parseFloat(coords[0].trim());
            const lon = parseFloat(coords[1].trim());
            if (!isNaN(lat) && !isNaN(lon)) {
                profileData.latitude = lat;
                profileData.longitude = lon;
            }
        }
    }

    // Ne pas envoyer de champs vides qui pourraient causer des erreurs
    Object.keys(profileData).forEach(key => {
        if (profileData[key] === '' && key !== 'biography') {
            delete profileData[key];
        }
    });


    try {
        const response = await fetch('/api/profile', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(profileData),
        });

        if (response.ok) {
            showNotification('Profil mis à jour avec succès', 'success');
            // Ne pas recharger, juste afficher le succès
        } else {
            const responseText = await response.text();
            
            try {
                const errorData = JSON.parse(responseText);
                if (errorData.errors && Array.isArray(errorData.errors)) {
                    // Afficher la première erreur de validation
                    showNotification(errorData.errors[0].message || 'Erreur de validation', 'error');
                } else {
                    showNotification(errorData.error || errorData.message || 'Erreur lors de la mise à jour', 'error');
                }
            } catch (parseError) {
                showNotification('Erreur: ' + responseText, 'error');
            }
        }
    } catch (error) {
        showNotification('Erreur de connexion lors de la mise à jour du profil', 'error');
    }
}

// Fonction pour ajouter un tag
async function addTag(e) {
    e.preventDefault();

    const tagInput = document.getElementById('new-tag');
    const tagName = tagInput.value.trim();

    if (!tagName) {
        showNotification('Veuillez entrer un tag', 'error');
        return;
    }

    try {
        const response = await fetch('/api/profile/tags', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ tag_name: tagName }),
        });

        if (response.ok) {
            const data = await response.json();
            tagInput.value = '';
            showNotification('Tag ajouté avec succès !', 'success');
            
            // Mettre à jour l'affichage immédiatement si possible
            if (data.tags) {
                updateTagsDisplay(data.tags);
            } else {
                // Si pas de tags dans la réponse, recharger après un délai
                setTimeout(() => location.reload(), 500);
            }
        } else {
            const responseText = await response.text();
            
            try {
                const jsonData = JSON.parse(responseText);
                showNotification(jsonData.error || jsonData.message || 'Erreur lors de l\'ajout du tag', 'error');
            } catch (parseError) {
                showNotification(responseText || 'Erreur lors de l\'ajout du tag', 'error');
            }
        }
        
    } catch (error) {
        showNotification('Erreur de connexion lors de l\'ajout du tag', 'error');
    }
}

// Fonction pour mettre à jour l'affichage des tags
function updateTagsDisplay(tags) {
    const tagsContainer = document.querySelector('.tags-container');
    if (!tagsContainer) return;
    
    let tagsHTML = '';
    if (tags && tags.length > 0) {
        tags.forEach(tag => {
            tagsHTML += `<span class="tag">${tag.name} <button class="remove-tag" data-id="${tag.id}">×</button></span>`;
        });
    } else {
        tagsHTML = '<p>Aucun intérêt ajouté</p>';
    }
    
    tagsContainer.innerHTML = tagsHTML;
}

// Fonction pour supprimer un tag
async function removeTag(tagId) {
    if (!confirm('Voulez-vous vraiment supprimer ce tag ?')) {
        return;
    }

    try {
        const response = await fetch(`/api/profile/tags/${tagId}`, {
            method: 'DELETE',
        });

        if (response.ok) {
            // Recharger la page pour actualiser les tags
            location.reload();
        } else {
            const data = await response.json();
            alert('Erreur : ' + (data.message || 'Erreur lors de la suppression du tag'));
        }
    } catch (error) {
        alert('Erreur lors de la suppression du tag');
    }
}

async function uploadPhoto(e) {
    e.preventDefault();

    const fileInput = document.getElementById('photo-file');
    const file = fileInput.files[0];
    
    if (!file) {
        showNotification('❌ Veuillez sélectionner une image', 'error');
        return;
    }
    
    // Validation côté client pour éviter les erreurs console
    const allowedTypes = ['image/jpeg', 'image/jpg', 'image/png', 'image/gif'];
    const fileExt = file.name.toLowerCase().split('.').pop();
    const allowedExts = ['jpg', 'jpeg', 'png', 'gif'];
    
    if (!allowedTypes.includes(file.type) || !allowedExts.includes(fileExt)) {
        showNotification(`❌ Format "${fileExt}" non supporté. Utilisez JPG, PNG ou GIF`, 'error');
        return;
    }
    
    if (file.size > 8 * 1024 * 1024) {
        const sizeMB = (file.size / (1024 * 1024)).toFixed(1);
        showNotification(`❌ Fichier trop volumineux (${sizeMB}MB). Maximum : 8MB`, 'error');
        return;
    }
    
    if (file.size === 0) {
        showNotification('❌ Le fichier est vide', 'error');
        return;
    }

    const formData = new FormData(e.target);
    showNotification('⏳ Upload en cours...', 'info', 0);

    try {
        const response = await fetch('/api/profile/photos', {
            method: 'POST',
            body: formData,
        });

        document.querySelector('.notification-info')?.remove();

        // LE FIX CRUCIAL : Gérer response.ok ET les erreurs dans le même try
        if (response.ok) {
            showNotification('✅ Photo uploadée avec succès !', 'success');
            e.target.reset();
            setTimeout(() => location.reload(), 1500);
        } else {
            // ERREUR HTTP (400, 500, etc.) - PAS une erreur réseau
            let errorMessage = 'Erreur lors de l\'upload';
            
            try {
                const data = await response.json();
                errorMessage = data.error || errorMessage;
                
                // Messages détaillés basés sur les erreurs du serveur Go
                if (errorMessage.includes('trop grande')) {
                    errorMessage = `❌ Image trop grande (max 5000x5000 pixels). ${errorMessage}`;
                } else if (errorMessage.includes('trop petite')) {
                    errorMessage = `❌ Image trop petite (min 50x50 pixels). ${errorMessage}`;
                } else if (errorMessage.includes('limite')) {
                    errorMessage = '❌ Limite de 5 photos atteinte. Supprimez une photo existante';
                } else if (errorMessage.includes('sécurité')) {
                    errorMessage = '❌ Image rejetée pour des raisons de sécurité';
                } else if (errorMessage.includes('volumineux')) {
                    errorMessage = `❌ ${errorMessage}`;
                } else {
                    errorMessage = `❌ ${errorMessage}`;
                }
                
            } catch (jsonError) {
                // Si pas de JSON, utiliser le status HTTP
                errorMessage = `❌ Erreur serveur (${response.status})`;
            }
            
            showNotification(errorMessage, 'error');
        }
        
    } catch (networkError) {
        // VRAIE erreur réseau (pas de réponse du serveur)
        document.querySelector('.notification-info')?.remove();
        showNotification('❌ Impossible de joindre le serveur. Vérifiez votre connexion', 'error');
    }
}

// Fonction auxiliaire pour validation des magic bytes (FONCTIONNE)
function validateMagicBytes(file) {
    return new Promise((resolve) => {
        const reader = new FileReader();
        reader.onload = function(e) {
            try {
                const uint8Array = new Uint8Array(e.target.result);
                
                if (uint8Array.length < 12) {
                    resolve({ valid: false, reason: "fichier trop petit pour contenir les headers d'image" });
                    return;
                }
                
                // PNG: 89 50 4E 47 0D 0A 1A 0A
                if (uint8Array[0] === 0x89 && uint8Array[1] === 0x50 && 
                    uint8Array[2] === 0x4E && uint8Array[3] === 0x47) {
                    resolve({ valid: true, format: 'PNG' });
                    return;
                }
                
                // JPEG: FF D8 FF
                if (uint8Array[0] === 0xFF && uint8Array[1] === 0xD8 && uint8Array[2] === 0xFF) {
                    resolve({ valid: true, format: 'JPEG' });
                    return;
                }
                
                // GIF: GIF87a or GIF89a
                if (uint8Array[0] === 0x47 && uint8Array[1] === 0x49 && uint8Array[2] === 0x46) {
                    resolve({ valid: true, format: 'GIF' });
                    return;
                }
                
                resolve({ valid: false, reason: "les bytes d'en-tête ne correspondent à aucun format d'image supporté (PNG, JPEG, GIF)" });
            } catch (error) {
                resolve({ valid: false, reason: "erreur lors de la lecture des headers" });
            }
        };
        reader.onerror = () => resolve({ valid: false, reason: "impossible de lire le fichier" });
        reader.readAsArrayBuffer(file.slice(0, 12));
    });
}

// Fonction auxiliaire pour obtenir les dimensions (FONCTIONNE)
function getSimpleImageDimensions(file) {
    return new Promise((resolve, reject) => {
        const img = new Image();
        
        img.onload = function() {
            resolve({
                width: this.naturalWidth,
                height: this.naturalHeight
            });
            URL.revokeObjectURL(img.src); // Nettoyer la mémoire
        };
        
        img.onerror = function() {
            reject(new Error('Impossible de charger l\'image pour lire ses dimensions'));
        };
        
        // Créer une URL temporaire pour l'image
        img.src = URL.createObjectURL(file);
    });
}

// Fonction pour supprimer une photo
async function deletePhoto(photoId) {
    try {
        const response = await fetch(`/api/profile/photos/${photoId}`, {
            method: 'DELETE',
        });

        if (response.ok) {
            const photoContainer = document.querySelector(`[data-id="${photoId}"]`);
            if (photoContainer) {
                photoContainer.style.opacity = '0.5';
                photoContainer.style.transition = 'opacity 0.3s';
                setTimeout(() => {
                    photoContainer.remove();
                    showNotification('✅ Photo supprimée avec succès', 'success');
                }, 300);
            } else {
                showNotification('✅ Photo supprimée avec succès', 'success');
                setTimeout(() => location.reload(), 1000);
            }
        } else {
            let errorMessage = 'Erreur lors de la suppression de la photo';
            try {
                const data = await response.json();
                errorMessage = data.error || data.message || errorMessage;
            } catch (parseError) {
                const textError = await response.text();
                errorMessage = textError || errorMessage;
            }
            showNotification(errorMessage, 'error');
        }
    } catch (error) {
        showNotification('Erreur de connexion lors de la suppression', 'error');
    }
}

// Ajouter cette fonction pour s'assurer que showNotification existe
function showNotification(message, type = 'info', duration = 5000) {
    // Supprimer les notifications existantes
    const existingNotifications = document.querySelectorAll('.notification-popup');
    existingNotifications.forEach(notif => notif.remove());

    // Créer la notification
    const notification = document.createElement('div');
    notification.className = `notification notification-${type}`;
    
    // Icônes selon le type
    const icons = {
        'success': '✅',
        'error': '❌', 
        'warning': '⚠️',
        'info': 'ℹ️'
    };
    
    // Couleurs selon le type
    const colors = {
        'success': { bg: '#10B981', border: '#059669' },
        'error': { bg: '#EF4444', border: '#DC2626' },
        'warning': { bg: '#F59E0B', border: '#D97706' },
        'info': { bg: '#3B82F6', border: '#2563EB' }
    };
    
    const color = colors[type] || colors.info;
    
    notification.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        background: ${color.bg};
        color: white;
        padding: 16px 20px;
        border-radius: 8px;
        border-left: 4px solid ${color.border};
        box-shadow: 0 10px 25px rgba(0,0,0,0.2);
        z-index: 10000;
        max-width: 400px;
        min-width: 300px;
        font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
        font-size: 14px;
        line-height: 1.5;
        animation: slideInRight 0.3s ease-out;
        cursor: pointer;
        display: flex;
        align-items: center;
        gap: 10px;
    `;
    
    notification.innerHTML = `
        <span style="font-size: 18px;">${icons[type] || icons.info}</span>
        <span style="flex: 1;">${message}</span>
        <button style="
            background: none;
            border: none;
            color: white;
            font-size: 18px;
            cursor: pointer;
            padding: 0;
            width: 20px;
            height: 20px;
            opacity: 0.7;
        " onclick="this.parentElement.remove()">×</button>
    `;

    // Ajouter les styles d'animation si ils n'existent pas
    if (!document.querySelector('#notification-styles')) {
        const style = document.createElement('style');
        style.id = 'notification-styles';
        style.textContent = `
            @keyframes slideInRight {
                from { transform: translateX(100%); opacity: 0; }
                to { transform: translateX(0); opacity: 1; }
            }
        `;
        document.head.appendChild(style);
    }

    // Ajouter au DOM
    document.body.appendChild(notification);

    // Auto-fermeture
    if (duration > 0) {
        setTimeout(() => {
            if (notification.parentElement) {
                notification.remove();
            }
        }, duration);
    }

    // Permettre de fermer en cliquant
    notification.addEventListener('click', () => notification.remove());
}

// Fonction pour définir la photo de profil
async function setProfilePhoto(photoId) {
    try {
        const response = await fetch(`/api/profile/photos/${photoId}/set-profile`, {
            method: 'PUT',
        });

        if (response.ok) {
            // Recharger la page pour actualiser l'affichage
            location.reload();
        } else {
            const data = await response.json();
            alert('Erreur : ' + (data.message || 'Erreur lors de la définition de la photo de profil'));
        }
    } catch (error) {
        alert('Erreur lors de la définition de la photo de profil');
    }
}

// Fonction principale pour mettre à jour la localisation (remplace l'ancienne)
async function updateLocation() {
    const button = document.getElementById('update-location');
    button.disabled = true;
    button.textContent = 'Localisation en cours...';

    try {
        const location = await getLocationWithFallback();
        await saveLocationToProfile(location);
        
        // Mettre à jour l'interface
        const locationField = document.getElementById('location');
        if (locationField) {
            if (location.city) {
                locationField.value = `${location.city} (${location.latitude.toFixed(4)}, ${location.longitude.toFixed(4)})`;
            } else {
                locationField.value = `${location.latitude.toFixed(4)}, ${location.longitude.toFixed(4)}`;
            }
        }
        
        showNotification(`✅ Localisation mise à jour avec succès${location.method ? ' (' + location.method + ')' : ''}`, 'success');
        
    } catch (error) {
        showNotification('❌ Impossible de déterminer votre localisation', 'error');
    } finally {
        button.disabled = false;
        button.textContent = 'Actualiser ma position';
    }
}

// Fonction principale de géolocalisation avec fallback automatique (CONFORME AU SUJET)
async function getLocationWithFallback() {
    // ÉTAPE 1: Essayer la géolocalisation GPS
    try {
        if (navigator.geolocation) {
            const gpsLocation = await getGPSLocation();
            gpsLocation.method = 'GPS';
            return gpsLocation;
        }
    } catch (gpsError) {
    }
    
    // ÉTAPE 2: Fallback automatique - Géolocalisation par IP (SANS PERMISSION)
    try {
        const ipLocation = await getIPLocation();
        ipLocation.method = 'IP automatique';
        return ipLocation;
    } catch (ipError) {
    }
    
    // ÉTAPE 3: Fallback ultime - Coordonnées par défaut
    return {
        latitude: 48.8566,
        longitude: 2.3522,
        city: 'Paris, France',
        method: 'Défaut',
        accuracy: null
    };
}

// Géolocalisation par IP - SANS PERMISSION (conforme au sujet)
async function getIPLocation() {
    const services = [
        {
            name: 'ipapi.co',
            url: 'https://ipapi.co/json/',
            parser: (data) => ({
                latitude: data.latitude,
                longitude: data.longitude,
                city: data.city && data.country_name ? `${data.city}, ${data.country_name}` : null,
                accuracy: 10000
            })
        },
        {
            name: 'freegeoip.app',
            url: 'https://freegeoip.app/json/',
            parser: (data) => ({
                latitude: data.latitude,
                longitude: data.longitude,
                city: data.city && data.country_name ? `${data.city}, ${data.country_name}` : null,
                accuracy: 10000
            })
        },
        {
            name: 'ipgeolocation.io',
            url: 'https://api.ipgeolocation.io/ipgeo?apiKey=free',
            parser: (data) => ({
                latitude: parseFloat(data.latitude),
                longitude: parseFloat(data.longitude),
                city: data.city && data.country_name ? `${data.city}, ${data.country_name}` : null,
                accuracy: 10000
            })
        }
    ];
    
    for (const service of services) {
        try {
            
            const controller = new AbortController();
            const timeoutId = setTimeout(() => controller.abort(), 5000);
            
            const response = await fetch(service.url, {
                method: 'GET',
                headers: {
                    'Accept': 'application/json'
                },
                signal: controller.signal
            });
            
            clearTimeout(timeoutId);
            
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            
            const data = await response.json();
            const location = service.parser(data);
            
            if (isValidCoordinates(location.latitude, location.longitude)) {
                return location;
            } else {
                throw new Error('Coordonnées invalides');
            }
            
        } catch (error) {
            continue;
        }
    }
    
    throw new Error('Tous les services de géolocalisation IP ont échoué');
}

// Validation des coordonnées
function isValidCoordinates(lat, lon) {
    return (
        typeof lat === 'number' && 
        typeof lon === 'number' && 
        !isNaN(lat) && 
        !isNaN(lon) && 
        lat >= -90 && 
        lat <= 90 && 
        lon >= -180 && 
        lon <= 180 &&
        !(lat === 0 && lon === 0) // Éviter les coordonnées nulles
    );
}

// Sauvegarder la localisation dans le profil
async function saveLocationToProfile(location) {
    const currentData = {
        gender: document.getElementById('gender')?.value || '',
        sexual_preference: document.getElementById('sexual_preference')?.value || '',
        biography: document.getElementById('biography')?.value || '',
        birth_date: document.getElementById('birth_date')?.value ? 
            document.getElementById('birth_date').value + 'T00:00:00Z' : null,
        latitude: location.latitude,
        longitude: location.longitude,
        location_name: location.city || `${location.latitude.toFixed(4)}, ${location.longitude.toFixed(4)}`
    };

    // Nettoyer les données vides
    Object.keys(currentData).forEach(key => {
        if (currentData[key] === '' && key !== 'biography') {
            delete currentData[key];
        }
    });

    const response = await fetch('/api/profile', {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(currentData),
    });

    if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Erreur serveur: ${errorText}`);
    }
}

// FONCTION CRUCIALE: Géolocalisation automatique lors du premier accès (CONFORME AU SUJET)
async function initializeLocationIfMissing() {
    const locationField = document.getElementById('location');
    if (!locationField || locationField.value.trim() !== '') {
        return; // L'utilisateur a déjà une localisation
    }
    
    
    try {
        // Géolocalisation silencieuse sans demander la permission GPS
        const location = await getSilentLocation();
        await saveLocationToProfile(location);
        
        // Mise à jour de l'interface
        if (location.city) {
            locationField.value = `${location.city} (${location.latitude.toFixed(4)}, ${location.longitude.toFixed(4)})`;
        } else {
            locationField.value = `${location.latitude.toFixed(4)}, ${location.longitude.toFixed(4)}`;
        }
        
        
        // Notification discrète
        if (location.method !== 'Défaut') {
            showNotification(`📍 Localisation détectée automatiquement`, 'info');
        }
        
    } catch (error) {
    }
}

// Géolocalisation GPS avec promesse et gestion des erreurs
function getGPSLocation() {
    return new Promise((resolve, reject) => {
        const options = {
            enableHighAccuracy: true,
            timeout: 8000, // 8 secondes max
            maximumAge: 300000 // Cache 5 minutes
        };
        
        navigator.geolocation.getCurrentPosition(
            (position) => {
                resolve({
                    latitude: position.coords.latitude,
                    longitude: position.coords.longitude,
                    accuracy: position.coords.accuracy,
                    city: null
                });
            },
            (error) => {
                let errorMessage = 'Erreur GPS';
                switch (error.code) {
                    case error.PERMISSION_DENIED:
                        errorMessage = 'Permission GPS refusée par l\'utilisateur';
                        break;
                    case error.POSITION_UNAVAILABLE:
                        errorMessage = 'Position GPS indisponible';
                        break;
                    case error.TIMEOUT:
                        errorMessage = 'Timeout GPS (8 secondes)';
                        break;
                }
                reject(new Error(errorMessage));
            },
            options
        );
    });
}

// Ajoutez aussi une fonction pour permettre la modification manuelle
function enableManualLocation() {
    const locationInput = document.getElementById('location');
    locationInput.readOnly = false;
    locationInput.placeholder = "Entrez votre ville ou coordonnées (ex: Paris ou 48.8566, 2.3522)";
}

// Géolocalisation silencieuse (SANS demander la permission - conforme au sujet)
async function getSilentLocation() {
    // Directement utiliser la géolocalisation IP
    try {
        const ipLocation = await getIPLocation();
        ipLocation.method = 'IP silencieux';
        return ipLocation;
    } catch (error) {
        // Fallback coordonnées par défaut
        return {
            latitude: 48.8566,
            longitude: 2.3522,
            city: 'Paris, France',
            method: 'Défaut',
            accuracy: null
        };
    }
}