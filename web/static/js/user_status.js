// user_status.js - Gestion de l'affichage du statut des utilisateurs

// Fonction pour charger et afficher le statut d'un utilisateur
async function loadUserStatus(userId) {
    try {
        // CORRECTION : Ajouter un timestamp pour éviter le cache
        const timestamp = new Date().getTime();
        const response = await fetch(`/api/profile/${userId}/status?_t=${timestamp}`, {
            // CORRECTION : Forcer la requête à ne pas utiliser le cache
            cache: 'no-cache',
            headers: {
                'Cache-Control': 'no-cache',
                'Pragma': 'no-cache'
            }
        });
        
        if (!response.ok) {
            return;
        }
        
        const data = await response.json();
        updateUserStatusDisplay(userId, data);
        
    } catch (error) {
        return null;
    }
}

// Fonction pour mettre à jour l'affichage du statut
function updateUserStatusDisplay(userId, statusData) {
    const statusElements = document.querySelectorAll(`#user-status-${userId}, .user-status-${userId}`);
    
    if (statusElements.length === 0) {
        return;
    }
    
    const { is_online, last_connection, last_connection_formatted } = statusData;
    
    statusElements.forEach(element => {
        if (is_online) {
            // CORRECTION : Vérifier si la dernière connexion est récente (moins de 5 minutes)
            const now = new Date();
            const lastConn = new Date(last_connection);
            const diffMinutes = (now - lastConn) / (1000 * 60);
            
            if (diffMinutes <= 5) {
                // Vraiment en ligne (activité récente)
                element.innerHTML = `
                    <span class="status-indicator online"></span>
                    <span class="status-text">En ligne maintenant</span>
                `;
                element.className = 'user-status online';
            } else {
                // Marqué en ligne mais pas d'activité récente
                element.innerHTML = `
                    <span class="status-indicator away"></span>
                    <span class="status-text">Actif il y a ${Math.round(diffMinutes)} min</span>
                `;
                element.className = 'user-status away';
            }
        } else {
            // Hors ligne
            const timeText = last_connection_formatted || 'Jamais connecté';
            element.innerHTML = `
                <span class="status-indicator offline"></span>
                <span class="status-text">Vu ${timeText}</span>
            `;
            element.className = 'user-status offline';
        }
    });
}

// Fonction pour charger tous les statuts visibles sur la page
function loadAllVisibleStatuses() {
    // Chercher tous les éléments avec un ID user-status-*
    const statusElements = document.querySelectorAll('[id^="user-status-"]');
    
    statusElements.forEach(element => {
        const userId = element.id.replace('user-status-', '');
        if (userId && !isNaN(parseInt(userId))) {
            loadUserStatus(parseInt(userId));
        }
    });
}

// Fonction pour actualiser périodiquement les statuts
function startStatusUpdateInterval() {
    // CORRECTION : Actualiser moins fréquemment pour éviter les requêtes excessives
    setInterval(() => {
        loadAllVisibleStatuses();
    }, 60000); // Toutes les 60 secondes
}

// CSS pour les indicateurs de statut
function injectStatusCSS() {
    const statusCSS = `
        .user-status {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
            font-size: 0.9rem;
            padding: 0.25rem 0.5rem;
            border-radius: 12px;
            background: rgba(255, 255, 255, 0.1);
        }

        .status-indicator {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            display: inline-block;
        }

        .status-indicator.online {
            background-color: #28a745;
            box-shadow: 0 0 4px rgba(40, 167, 69, 0.6);
            animation: pulse-online 2s infinite;
        }

        .status-indicator.away {
            background-color: #ffc107;
            box-shadow: 0 0 4px rgba(255, 193, 7, 0.6);
        }

        .status-indicator.offline {
            background-color: #6c757d;
        }

        .user-status.online {
            background: rgba(40, 167, 69, 0.1);
            color: #28a745;
        }

        .user-status.away {
            background: rgba(255, 193, 7, 0.1);
            color: #856404;
        }

        .user-status.offline {
            background: rgba(108, 117, 125, 0.1);
            color: #6c757d;
        }

        .status-text {
            font-weight: 500;
        }

        @keyframes pulse-online {
            0% {
                box-shadow: 0 0 4px rgba(40, 167, 69, 0.6);
            }
            50% {
                box-shadow: 0 0 8px rgba(40, 167, 69, 0.8);
            }
            100% {
                box-shadow: 0 0 4px rgba(40, 167, 69, 0.6);
            }
        }

        /* Styles pour la page de profil */
        .profile-status {
            margin: 1rem 0;
            padding: 0.5rem;
            border-radius: 8px;
            background: rgba(255, 255, 255, 0.05);
        }

        /* Responsive */
        @media (max-width: 768px) {
            .user-status {
                font-size: 0.8rem;
                gap: 0.25rem;
            }
            
            .status-indicator {
                width: 6px;
                height: 6px;
            }
        }
    `;

    const styleSheet = document.createElement("style");
    styleSheet.textContent = statusCSS;
    document.head.appendChild(styleSheet);
}

// Fonction utilitaire pour formater le temps relatif
function formatRelativeTime(date) {
    const now = new Date();
    const diff = now - date;
    const minutes = Math.floor(diff / (1000 * 60));
    const hours = Math.floor(diff / (1000 * 60 * 60));
    const days = Math.floor(diff / (1000 * 60 * 60 * 24));

    if (minutes < 1) {
        return "à l'instant";
    } else if (minutes < 60) {
        return `il y a ${minutes} min`;
    } else if (hours < 24) {
        return `il y a ${hours}h`;
    } else if (days < 7) {
        return `il y a ${days}j`;
    } else {
        return date.toLocaleDateString('fr-FR');
    }
}

// Fonction pour observer les nouveaux éléments de statut ajoutés dynamiquement
function observeNewStatusElements() {
    const observer = new MutationObserver((mutations) => {
        mutations.forEach((mutation) => {
            mutation.addedNodes.forEach((node) => {
                if (node.nodeType === Node.ELEMENT_NODE) {
                    // Chercher les nouveaux éléments de statut
                    const newStatusElements = node.querySelectorAll ? 
                        node.querySelectorAll('[id^="user-status-"]') : [];
                    
                    newStatusElements.forEach(element => {
                        const userId = element.id.replace('user-status-', '');
                        if (userId && !isNaN(parseInt(userId))) {
                            setTimeout(() => {
                                loadUserStatus(parseInt(userId));
                            }, 500);
                        }
                    });
                }
            });
        });
    });

    observer.observe(document.body, {
        childList: true,
        subtree: true
    });
}

// Initialisation
document.addEventListener('DOMContentLoaded', function() {
    injectStatusCSS();
    
    // Charger les statuts immédiatement
    setTimeout(() => {
        loadAllVisibleStatuses();
    }, 1000); // Délai de 1 seconde pour laisser la page se charger
    
    // Démarrer les mises à jour périodiques
    startStatusUpdateInterval();
    
    // Démarrer l'observation des nouveaux éléments
    observeNewStatusElements();
});

// Forcer le nettoyage du cache au chargement de la page
window.addEventListener('pageshow', function(event) {
    if (event.persisted) {
        // Page restaurée depuis le cache, recharger les statuts
        setTimeout(() => {
            loadAllVisibleStatuses();
        }, 500);
    }
});