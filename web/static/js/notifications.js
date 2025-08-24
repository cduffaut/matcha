// Gestion des notifications
document.addEventListener('DOMContentLoaded', function() {
    const markAllReadBtn = document.getElementById('mark-all-read');
    const notificationItems = document.querySelectorAll('.notification-item');

    // Marquer toutes les notifications comme lues
    if (markAllReadBtn) {
        markAllReadBtn.addEventListener('click', async function() {
            try {
                const response = await fetch('/api/notifications/mark-all-read', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    }
                });

                if (response.ok) {
                    // Marquer visuellement toutes les notifications comme lues
                    notificationItems.forEach(item => {
                        item.classList.remove('unread');
                    });
                    
                    // Optionnel: recharger la page pour refléter les changements
                    window.location.reload();
                } else {
                    return null;
                }
            } catch (error) {
                return null;
            }
        });
    }

    // Marquer une notification individuelle comme lue au clic
    notificationItems.forEach(item => {
        item.addEventListener('click', async function() {
            const notificationId = this.dataset.id;
            const isUnread = this.classList.contains('unread');

            if (isUnread && notificationId) {
                try {
                    const response = await fetch(`/api/notifications/${notificationId}/read`, {
                        method: 'PUT',
                        headers: {
                            'Content-Type': 'application/json'
                        }
                    });

                    if (response.ok) {
                        this.classList.remove('unread');
                        updateNotificationCount();
                    }
                } catch (error) {
                    return null;
                }
            }
        });
    });

    // Fonction pour mettre à jour le compteur de notifications
    function updateNotificationCount() {
        fetch('/api/notifications/unread-count')
            .then(response => response.json())
            .then(data => {
                const countElements = document.querySelectorAll('#notification-count');
                countElements.forEach(element => {
                    if (data.unread_count > 0) {
                        element.textContent = `(${data.unread_count})`;
                        element.style.color = 'red';
                        element.style.fontWeight = 'bold';
                    } else {
                        element.textContent = '';
                    }
                });
            })
            .catch(error => {
                return null;
            });
    }

    // Charger le compteur au démarrage
    updateNotificationCount();

    // Optionnel: actualiser le compteur périodiquement
    setInterval(updateNotificationCount, 30000); // Toutes les 30 secondes
});

// Fonction utilitaire pour formater les dates
function formatDate(dateString) {
    const date = new Date(dateString);
    const now = new Date();
    const diffTime = Math.abs(now - date);
    const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

    if (diffDays === 1) {
        return 'Il y a 1 jour';
    } else if (diffDays < 7) {
        return `Il y a ${diffDays} jours`;
    } else {
        return date.toLocaleDateString('fr-FR');
    }
}