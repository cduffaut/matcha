// ============================================================================
// AUTH.JS - VERSION MISE À JOUR POUR MATCHA
// Utilise le gestionnaire global d'erreurs pour zéro erreur console
// ============================================================================

document.addEventListener('DOMContentLoaded', function() {
    // Gestion du formulaire d'inscription
    const registerForm = document.getElementById('register-form');
    if (registerForm) {
        registerForm.addEventListener('submit', handleRegister);
    }

    // Gestion du formulaire de connexion
    const loginForm = document.getElementById('login-form');
    if (loginForm) {
        loginForm.addEventListener('submit', handleLogin);
    }
    
    // Gestion du formulaire de mot de passe oublié
    const forgotPasswordForm = document.getElementById('forgot-password-form');
    if (forgotPasswordForm) {
        forgotPasswordForm.addEventListener('submit', handleForgotPassword);
    }
});

// ============================================================================
// FONCTION DE CONNEXION - ZÉRO ERREUR CONSOLE
// ============================================================================
async function handleLogin(e) {
    e.preventDefault();
    
    const username = document.getElementById('username').value.trim();
    const password = document.getElementById('password').value;
    
    // Validation côté client
    if (!username) {
        showError('Veuillez saisir votre nom d\'utilisateur');
        return;
    }
    
    if (!password) {
        showError('Veuillez saisir votre mot de passe');
        return;
    }
    
    // Désactiver le bouton pendant la requête
    const submitButton = e.target.querySelector('button[type="submit"]');
    const originalText = submitButton ? submitButton.textContent : '';
    if (submitButton) {
        submitButton.disabled = true;
        submitButton.textContent = 'Connexion...';
    }
    
    try {
        // Utiliser la fonction globale handleFormSubmission
        const result = await window.handleFormSubmission('/api/login', {
            username: username,
            password: password
        }, {
            redirectOnSuccess: '/profile',
            onSuccess: (data) => {
                showSuccess('Connexion réussie !');
            },
            onError: (error) => {
                // L'erreur est déjà affichée par handleFormSubmission
                console.log('Erreur de connexion gérée:', error);
            }
        });
        
    } finally {
        // Réactiver le bouton
        if (submitButton) {
            submitButton.disabled = false;
            submitButton.textContent = originalText;
        }
    }
}

// ============================================================================
// FONCTION D'INSCRIPTION - ZÉRO ERREUR CONSOLE
// ============================================================================
async function handleRegister(e) {
    e.preventDefault();
    
    const username = document.getElementById('username').value.trim();
    const email = document.getElementById('email').value.trim();
    const firstName = document.getElementById('first_name').value.trim();
    const lastName = document.getElementById('last_name').value.trim();
    const password = document.getElementById('password').value;
    const confirmPassword = document.getElementById('confirm_password')?.value;
    
    // Validation côté client
    if (!username || !email || !firstName || !lastName || !password) {
        showError('Veuillez remplir tous les champs obligatoires');
        return;
    }
    
    if (confirmPassword && password !== confirmPassword) {
        showError('Les mots de passe ne correspondent pas');
        return;
    }
    
    if (!isValidEmail(email)) {
        showError('Veuillez saisir un email valide');
        return;
    }
    
    if (password.length < 8) {
        showError('Le mot de passe doit contenir au moins 8 caractères');
        return;
    }
    
    // Désactiver le bouton
    const submitButton = e.target.querySelector('button[type="submit"]');
    const originalText = submitButton ? submitButton.textContent : '';
    if (submitButton) {
        submitButton.disabled = true;
        submitButton.textContent = 'Inscription...';
    }
    
    try {
        const result = await window.handleFormSubmission('/api/register', {
            username: username,
            email: email,
            first_name: firstName,
            last_name: lastName,
            password: password
        }, {
            onSuccess: (data) => {
                showSuccess('Inscription réussie ! Vérifiez votre email pour activer votre compte.');
                // Optionnel : rediriger vers la page de login après quelques secondes
                setTimeout(() => {
                    window.location.href = '/login?registered=true';
                }, 2000);
            },
            onError: (error) => {
                console.log('Erreur d\'inscription gérée:', error);
            }
        });
        
    } finally {
        if (submitButton) {
            submitButton.disabled = false;
            submitButton.textContent = originalText;
        }
    }
}

// ============================================================================
// FONCTION MOT DE PASSE OUBLIÉ
// ============================================================================
async function handleForgotPassword(e) {
    e.preventDefault();
    
    const email = document.getElementById('email').value.trim();
    
    if (!email) {
        showError('Veuillez saisir votre adresse email');
        return;
    }
    
    if (!isValidEmail(email)) {
        showError('Veuillez saisir un email valide');
        return;
    }
    
    const submitButton = e.target.querySelector('button[type="submit"]');
    const originalText = submitButton ? submitButton.textContent : '';
    if (submitButton) {
        submitButton.disabled = true;
        submitButton.textContent = 'Envoi...';
    }
    
    try {
        const result = await window.handleFormSubmission('/api/forgot-password', {
            email: email
        }, {
            onSuccess: (data) => {
                showSuccess('Instructions de réinitialisation envoyées par email');
                // Optionnel : masquer le formulaire ou rediriger
                setTimeout(() => {
                    window.location.href = '/login?reset-sent=true';
                }, 2000);
            },
            onError: (error) => {
                console.log('Erreur mot de passe oublié gérée:', error);
            }
        });
        
    } finally {
        if (submitButton) {
            submitButton.disabled = false;
            submitButton.textContent = originalText;
        }
    }
}

// ============================================================================
// FONCTIONS UTILITAIRES
// ============================================================================

function isValidEmail(email) {
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    return emailRegex.test(email);
}

// Fonction pour gérer les messages d'URL (ex: ?registered=true)
function handleURLMessages() {
    const urlParams = new URLSearchParams(window.location.search);
    
    if (urlParams.get('registered') === 'true') {
        showSuccess('Inscription réussie ! Connectez-vous avec vos identifiants.');
    }
    
    if (urlParams.get('verified') === 'true') {
        showSuccess('Email vérifié avec succès ! Vous pouvez maintenant vous connecter.');
    }
    
    if (urlParams.get('reset-sent') === 'true') {
        showSuccess('Email de réinitialisation envoyé !');
    }
}

// Appeler au chargement de la page
document.addEventListener('DOMContentLoaded', handleURLMessages);