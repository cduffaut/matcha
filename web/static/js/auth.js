// Gérer les formulaires d'authentification
document.addEventListener('DOMContentLoaded', function() {
    const registerForm = document.getElementById('register-form');
    const loginForm = document.getElementById('login-form');

    if (registerForm) {
        registerForm.addEventListener('submit', handleRegister);
    }

    if (loginForm) {
        loginForm.addEventListener('submit', handleLogin);
    }

    // Vérifier les paramètres d'URL pour afficher des messages
    const urlParams = new URLSearchParams(window.location.search);
    if (urlParams.get('verified') === 'true') {
        showSuccess('Email vérifié avec succès ! Vous pouvez maintenant vous connecter.');
    }
});

async function handleRegister(e) {
    e.preventDefault();
    
    const formData = {
        username: document.getElementById('username').value,
        email: document.getElementById('email').value,
        first_name: document.getElementById('first_name').value,
        last_name: document.getElementById('last_name').value,
        password: document.getElementById('password').value
    };

    try {
        const response = await fetch('/api/register', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(formData),
        });

        const data = await response.json();

        if (response.ok) {
            showSuccess('Inscription réussie ! Veuillez vérifier votre email.');
            // Réinitialiser le formulaire
            e.target.reset();
        } else {
            showError(data.error || 'Erreur lors de l\'inscription');
        }
    } catch (error) {
        console.error('Erreur:', error);
        showError('Erreur lors de l\'inscription');
    }
}

async function handleLogin(e) {
    e.preventDefault();
    
    const formData = {
        username: document.getElementById('username').value,
        password: document.getElementById('password').value
    };

    try {
        const response = await fetch('/api/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(formData),
        });

        const data = await response.json();

        if (response.ok) {
            // Rediriger vers la page de profil
            window.location.href = '/profile';
        } else {
            showError(data.message || 'Nom d\'utilisateur ou mot de passe incorrect');
        }
    } catch (error) {
        console.error('Erreur:', error);
        showError('Erreur lors de la connexion');
    }
}

function showError(message) {
    // Créer ou obtenir l'élément d'erreur
    let errorDiv = document.querySelector('.error');
    if (!errorDiv) {
        errorDiv = document.createElement('div');
        errorDiv.className = 'error';
        document.querySelector('.container').insertBefore(errorDiv, document.querySelector('form'));
    }
    
    errorDiv.textContent = message;
    errorDiv.style.display = 'block';

    // Masquer après 5 secondes
    setTimeout(() => {
        errorDiv.style.display = 'none';
    }, 5000);
}

function showSuccess(message) {
    // Créer ou obtenir l'élément de succès
    let successDiv = document.querySelector('.success');
    if (!successDiv) {
        successDiv = document.createElement('div');
        successDiv.className = 'success';
        document.querySelector('.container').insertBefore(successDiv, document.querySelector('form'));
    }
    
    successDiv.textContent = message;
    successDiv.style.display = 'block';

    // Masquer après 5 secondes
    setTimeout(() => {
        successDiv.style.display = 'none';
    }, 5000);
}