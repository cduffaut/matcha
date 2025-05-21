// Gérer les mises à jour du profil
document.addEventListener('DOMContentLoaded', function() {
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

    // Gérer la suppression de photos
    document.addEventListener('click', function(e) {
        if (e.target.classList.contains('delete-photo')) {
            const photoContainer = e.target.closest('.photo-container');
            const photoId = photoContainer.dataset.id;
            deletePhoto(photoId);
        }
    });

    // Gérer la définition de la photo de profil
    document.addEventListener('click', function(e) {
        if (e.target.classList.contains('set-profile-photo')) {
            const photoContainer = e.target.closest('.photo-container');
            const photoId = photoContainer.dataset.id;
            setProfilePhoto(photoId);
        }
    });

    // Gérer la mise à jour de la localisation
    const updateLocationBtn = document.getElementById('update-location');
    if (updateLocationBtn) {
        updateLocationBtn.addEventListener('click', updateLocation);
    }
});

// Fonction pour sauvegarder le profil
async function saveProfile(e) {
    e.preventDefault();

    // Récupérer et formater la date de naissance
    const birthDateInput = document.getElementById('birth_date').value;
    let birthDate = null;
    if (birthDateInput) {
        // Créer un objet Date pour la date de naissance
        birthDate = new Date(birthDateInput);
        // Ajuster pour le fuseau horaire (éviter les décalages)
        birthDate.setHours(12);
    }

    // Récupérer les autres informations du profil
    const profileData = {
        gender: document.getElementById('gender').value,
        sexual_preference: document.getElementById('sexual_preference').value,
        biography: document.getElementById('biography').value,
        birth_date: birthDate,
        // Conserver les informations de localisation si elles existent
        location_name: document.getElementById('location')?.value || ""
    };

    // Récupérer les données de localisation à partir du nom de la localisation
    const locationParts = profileData.location_name.split('(');
    if (locationParts.length > 1) {
        const coordsStr = locationParts[1].replace(')', '').split(',');
        profileData.latitude = parseFloat(coordsStr[0].trim());
        profileData.longitude = parseFloat(coordsStr[1].trim());
    }

    try {
        const response = await fetch('/api/profile', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(profileData),
        });

        if (response.ok) {
            alert('Profil mis à jour avec succès');
        } else {
            const data = await response.json();
            alert('Erreur : ' + (data.message || 'Erreur lors de la mise à jour du profil'));
        }
    } catch (error) {
        console.error('Erreur:', error);
        alert('Erreur lors de la mise à jour du profil');
    }
}

// Fonction pour ajouter un tag
async function addTag(e) {
    e.preventDefault();

    const tagInput = document.getElementById('new-tag');
    const tagName = tagInput.value.trim();

    if (!tagName) {
        alert('Veuillez entrer un tag');
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
            tagInput.value = '';
            // Recharger les tags
            location.reload();
        } else {
            const data = await response.json();
            alert('Erreur : ' + (data.message || 'Erreur lors de l\'ajout du tag'));
        }
    } catch (error) {
        console.error('Erreur:', error);
        alert('Erreur lors de l\'ajout du tag');
    }
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
        console.error('Erreur:', error);
        alert('Erreur lors de la suppression du tag');
    }
}

// Fonction pour uploader une photo
async function uploadPhoto(e) {
    e.preventDefault();

    const formData = new FormData(e.target);

    try {
        const response = await fetch('/api/profile/photos', {
            method: 'POST',
            body: formData,
        });

        if (response.ok) {
            // Recharger la page pour afficher la nouvelle photo
            location.reload();
        } else {
            const data = await response.json();
            alert('Erreur : ' + (data.message || 'Erreur lors de l\'upload de la photo'));
        }
    } catch (error) {
        console.error('Erreur:', error);
        alert('Erreur lors de l\'upload de la photo');
    }
}

// Fonction pour supprimer une photo
async function deletePhoto(photoId) {
    if (!confirm('Voulez-vous vraiment supprimer cette photo ?')) {
        return;
    }

    try {
        const response = await fetch(`/api/profile/photos/${photoId}`, {
            method: 'DELETE',
        });

        if (response.ok) {
            // Recharger la page pour actualiser les photos
            location.reload();
        } else {
            const data = await response.json();
            alert('Erreur : ' + (data.message || 'Erreur lors de la suppression de la photo'));
        }
    } catch (error) {
        console.error('Erreur:', error);
        alert('Erreur lors de la suppression de la photo');
    }
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
        console.error('Erreur:', error);
        alert('Erreur lors de la définition de la photo de profil');
    }
}

// Fonction pour mettre à jour la localisation
async function updateLocation() {
    if (!navigator.geolocation) {
        alert('La géolocalisation n\'est pas supportée par votre navigateur');
        return;
    }

    const button = document.getElementById('update-location');
    button.disabled = true;
    button.textContent = 'Localisation en cours...';

    navigator.geolocation.getCurrentPosition(
        async function(position) {
            const latitude = position.coords.latitude;
            const longitude = position.coords.longitude;

            try {
                // Mettre à jour la localisation sur le serveur
                const response = await fetch('/api/profile', {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        latitude: latitude,
                        longitude: longitude,
                        location_name: `${latitude.toFixed(2)}, ${longitude.toFixed(2)}`,
                    }),
                });

                if (response.ok) {
                    const profile = await response.json();
                    // Mettre à jour l'affichage
                    document.getElementById('location').value = profile.location_name || `${latitude.toFixed(2)}, ${longitude.toFixed(2)}`;
                    alert('Localisation mise à jour avec succès');
                } else {
                    throw new Error('Erreur serveur');
                }
            } catch (error) {
                console.error('Erreur:', error);
                alert('Erreur lors de la mise à jour de la localisation');
            } finally {
                button.disabled = false;
                button.textContent = 'Mettre à jour ma position';
            }
        },
        function(error) {
            console.error('Erreur de géolocalisation:', error);
            alert('Impossible d\'obtenir votre position');
            button.disabled = false;
            button.textContent = 'Mettre à jour ma position';
        }
    );
}
