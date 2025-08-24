// Gestion automatique de l'onglet actif dans la navigation
document.addEventListener('DOMContentLoaded', function() {
    setActiveNavigation();
});

function setActiveNavigation() {
    // Déterminer la page courante
    const currentPath = window.location.pathname;
    const navLinks = document.querySelectorAll('nav a, header nav a');
    
    // Supprimer toutes les classes active
    navLinks.forEach(link => {
        link.classList.remove('active');
    });
    
    // Ajouter la classe active au lien correspondant
    navLinks.forEach(link => {
        const href = link.getAttribute('href');
        
        // Correspondances exactes et par préfixe
        if (href === currentPath || 
            (currentPath === '/profile' && href === '/profile') ||
            (currentPath.startsWith('/profile/') && href === '/profile') ||
            (currentPath === '/browse' && href === '/browse') ||
            (currentPath === '/notifications' && href === '/notifications') ||
            (currentPath === '/chat' && href === '/chat')) {
            
            link.classList.add('active');
        }
    });
}

// Réappliquer lors des changements de page (si navigation dynamique)
window.addEventListener('popstate', setActiveNavigation);