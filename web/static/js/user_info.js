// JavaScript pour la modification des informations utilisateur (nom, prénom, email)
document.addEventListener('DOMContentLoaded', function() {
    // Gérer la soumission du formulaire des informations utilisateur
    const updateUserInfoBtn = document.getElementById('update-user-info');
    if (updateUserInfoBtn) {
        updateUserInfoBtn.addEventListener('click', updateUserInfo);
    }
});

// Fonction pour mettre à jour les informations utilisateur
async function updateUserInfo(e) {
    e.preventDefault();

    // Récupérer les valeurs des champs
    const firstName = document.getElementById('first_name').value.trim();
    const lastName = document.getElementById('last_name').value.trim();
    const email = document.getElementById('email').value.trim();

    // Validation côté client
    if (!firstName) {
        showNotification('Le prénom est obligatoire', 'error');
        return;
    }

    if (!lastName) {
        showNotification('Le nom est obligatoire', 'error');
        return;
    }

    if (!email) {
        showNotification('L\'email est obligatoire', 'error');
        return;
    }

    // Validation email basique
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!emailRegex.test(email)) {
        showNotification('Format d\'email invalide', 'error');
        return;
    }

    // Préparer les données
    const userInfoData = {
        first_name: firstName,
        last_name: lastName,
        email: email
    };

    try {
        const response = await fetch('/api/user/update', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(userInfoData),
        });

        const data = await response.json();

        if (response.ok) {
            showNotification('Informations mises à jour avec succès !', 'success');
        } else {
            showNotification(data.error || 'Erreur lors de la mise à jour', 'error');
        }
    } catch (error) {
        showNotification('Erreur de connexion lors de la mise à jour', 'error');
    }
}

// Fonction pour afficher les notifications (réutilise la même que profile.js)
function showNotification(message, type) {
    // Créer l'élément notification
    const notification = document.createElement('div');
    notification.className = `notification ${type}`;
    notification.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        background-color: ${type === 'success' ? '#4CAF50' : '#f44336'};
        color: white;
        padding: 1rem 1.5rem;
        border-radius: 4px;
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
        z-index: 1000;
        cursor: pointer;
        transition: opacity 0.3s;
    `;
    notification.textContent = message;

    // Ajouter au DOM
    document.body.appendChild(notification);

    // Supprimer après 5 secondes
    setTimeout(() => notification.remove(), 5000);
    
    // Supprimer en cliquant
    notification.addEventListener('click', () => notification.remove());
}