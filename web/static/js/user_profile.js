// Fonctions utilitaires pour les messages
function showSuccessMessage(message) {
    // Supprimer les anciens messages
    const existingMessages = document.querySelectorAll('.success-message, .error-message');
    existingMessages.forEach(msg => msg.remove());

    const messageDiv = document.createElement('div');
    messageDiv.className = 'success-message';
    messageDiv.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        background-color: #4CAF50;
        color: white;
        padding: 1rem;
        border-radius: 4px;
        z-index: 1000;
        max-width: 300px;
        box-shadow: 0 2px 4px rgba(0,0,0,0.2);
    `;
    messageDiv.textContent = message;
    
    document.body.appendChild(messageDiv);
    
    setTimeout(() => {
        messageDiv.remove();
    }, 3000);
}

function showErrorMessage(message) {
    // Supprimer les anciens messages
    const existingMessages = document.querySelectorAll('.success-message, .error-message');
    existingMessages.forEach(msg => msg.remove());

    const messageDiv = document.createElement('div');
    messageDiv.className = 'error-message';
    messageDiv.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        background-color: #f44336;
        color: white;
        padding: 1rem;
        border-radius: 4px;
        z-index: 1000;
        max-width: 300px;
        box-shadow: 0 2px 4px rgba(0,0,0,0.2);
    `;
    messageDiv.textContent = message;
    
    document.body.appendChild(messageDiv);
    
    setTimeout(() => {
        messageDiv.remove();
    }, 5000);
}

// Fonction pour liker un utilisateur
async function likeUser(userId) {
    if (!userId) {
        showErrorMessage('ID utilisateur manquant');
        return;
    }

    try {
        const response = await fetch(`/api/profile/${userId}/like`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        });

        if (response.ok) {
            const data = await response.json();
            showSuccessMessage('✅ ' + data.message);
            
            // Mettre à jour l'interface
            const likeButton = document.querySelector(`button[onclick="likeUser(${userId})"]`);
            if (likeButton) {
                likeButton.outerHTML = `<button onclick="unlikeUser(${userId})" class="unlike-button" type="button">💔 Ne plus liker</button>`;
            }
            
            // Afficher le bouton de chat si c'est un match
            if (data.matched) {
                const actionsContainer = document.querySelector('.profile-actions');
                if (actionsContainer && !document.querySelector(`button[onclick="openChat(${userId})"]`)) {
                    const chatButton = document.createElement('button');
                    chatButton.onclick = () => openChat(userId);
                    chatButton.className = 'chat-button';
                    chatButton.type = 'button';
                    chatButton.textContent = '💬 Discuter';
                    actionsContainer.insertBefore(chatButton, actionsContainer.children[1]);
                }
                showSuccessMessage('🎉 C\'est un match ! Vous pouvez maintenant discuter.');
            }
        } else {
            const data = await response.json();
            showErrorMessage(data.error || 'Erreur lors du like');
        }
    } catch (error) {
        showErrorMessage('Erreur de connexion lors du like');
    }
}

// Fonction pour unliker un utilisateur
async function unlikeUser(userId) {
    if (!userId) {
        showErrorMessage('ID utilisateur manquant');
        return;
    }

    try {
        const response = await fetch(`/api/profile/${userId}/like`, {
            method: 'DELETE',
            headers: {
                'Content-Type': 'application/json'
            }
        });

        if (response.ok) {
            const data = await response.json();
            showSuccessMessage('✅ ' + data.message);
            
            // Mettre à jour l'interface
            const unlikeButton = document.querySelector(`button[onclick="unlikeUser(${userId})"]`);
            if (unlikeButton) {
                unlikeButton.outerHTML = `<button onclick="likeUser(${userId})" class="like-button" type="button">👍 Liker</button>`;
            }
            
            // Supprimer le bouton de chat s'il existe
            const chatButton = document.querySelector(`button[onclick="openChat(${userId})"]`);
            if (chatButton) {
                chatButton.remove();
            }
        } else {
            const data = await response.json();
            showErrorMessage(data.error || 'Erreur lors de l\'unlike');
        }
    } catch (error) {
        showErrorMessage('Erreur de connexion lors de l\'unlike');
    }
}

// Fonction pour ouvrir le chat
function openChat(userId) {
    if (!userId) {
        showErrorMessage('ID utilisateur manquant');
        return;
    }

    // Rediriger vers la page de chat avec l'utilisateur spécifique
    window.location.href = `/chat?user=${userId}`;
}

// Fonction pour bloquer un utilisateur
async function blockUser(userId) {
    if (!userId) {
        showErrorMessage('ID utilisateur manquant');
        return;
    }

    if (!confirm('Êtes-vous sûr de vouloir bloquer cet utilisateur ? Il ne pourra plus vous voir ni vous contacter.')) {
        return;
    }

    try {
        const response = await fetch(`/api/profile/${userId}/block`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        });

        if (response.ok) {
            const data = await response.json();
            showSuccessMessage('✅ ' + data.message);
            
            // Rediriger vers la page de recherche après un délai
            setTimeout(() => {
                window.location.href = '/browse';
            }, 2000);
        } else {
            const data = await response.json();
            showErrorMessage(data.error || 'Erreur lors du blocage');
        }
    } catch (error) {
        showErrorMessage('Erreur de connexion lors du blocage');
    }
}

// Fonction pour signaler un utilisateur
async function reportUser(userId) {
    if (!userId) {
        showErrorMessage('ID utilisateur manquant');
        return;
    }

    const reason = prompt('Pourquoi signalez-vous cet utilisateur ?');
    if (!reason || reason.trim().length === 0) {
        showErrorMessage('Raison du signalement requise');
        return;
    }

    try {
        const response = await fetch(`/api/profile/${userId}/report`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                reason: reason.trim()
            })
        });

        if (response.ok) {
            const data = await response.json();
            showSuccessMessage('✅ ' + data.message);
        } else {
            const data = await response.json();
            showErrorMessage(data.error || 'Erreur lors du signalement');
        }
    } catch (error) {
        showErrorMessage('Erreur de connexion lors du signalement');
    }
}

// Fonction à appeler quand on charge une page de profil utilisateur
function initUserProfileStatus() {
    // Récupérer l'ID utilisateur de la page courante
    const pathParts = window.location.pathname.split('/');
    if (pathParts[1] === 'profile' && pathParts[2]) {
        const userId = parseInt(pathParts[2]);
        
        if (userId && !isNaN(userId)) {
            // Trouver le conteneur de statut existant
            let statusContainer = document.querySelector('.online-status');
            
            if (statusContainer && window.userStatusManager) {
                // Utiliser le manager pour charger le statut
                window.userStatusManager.loadUserStatus(userId, statusContainer);
                
                // Mettre à jour périodiquement (toutes les 30 secondes)
                setInterval(() => {
                    window.userStatusManager.loadUserStatus(userId, statusContainer);
                }, 30000);
            }
        }
    }
}

// Auto-initialisation
document.addEventListener('DOMContentLoaded', function() {
    // Attendre que le manager soit initialisé
    setTimeout(initUserProfileStatus, 500);
});