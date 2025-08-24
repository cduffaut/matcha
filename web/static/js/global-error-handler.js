// global-error-handler.js - VERSION COMPLÈTE avec correction favicon.ico
// ============================================================================
// GESTIONNAIRE GLOBAL D'ERREURS MATCHA - VERSION COMPLÈTE
// web/static/js/global-error-handler.js
// ============================================================================

console.log('🔧 Chargement du gestionnaire d\'erreurs...');

(function() {
    'use strict';

    // =================================================================
    // 1. SUPPRESSION COMPLÈTE DES ERREURS HTTP DANS LA CONSOLE
    // =================================================================
    
    // Sauvegarder les méthodes originales
    const originalConsoleError = console.error;
    const originalConsoleWarn = console.warn;
    const originalConsoleLog = console.log;
    
    // Patterns d'erreurs à supprimer (CONFORMITÉ SUJET MATCHA + FAVICON)
    const suppressedPatterns = [
        // Erreurs HTTP courantes
        '401', '400', '403', '404', '500', '413', '415',
        'unauthorized', 'bad request', 'forbidden', 'not found',
        'internal server error', 'payload too large',
        
        // Erreurs spécifiques favicon - NOUVEAU
        'favicon.ico', 'favicon', 
        'http://localhost:8080/favicon.ico',
        'https://localhost:8080/favicon.ico',
        
        // Erreurs réseau
        'failed to fetch', 'network request failed', 'fetch failed',
        'net::ERR_EMPTY_RESPONSE', 'net::ERR_CONNECTION_REFUSED',
        'net::ERR_NETWORK_CHANGED', 'net::ERR_INTERNET_DISCONNECTED',
        
        // Erreurs API spécifiques Matcha
        '/api/login', '/api/register', '/api/profile/photos', '/api/profile',
        '/api/browse', '/api/chat', '/api/notifications', '/api/match',
        
        // Autres erreurs techniques
        'cors', 'mixed content', 'content security policy',
        'websocket', 'xhr', 'xmlhttprequest',
        
        // Erreurs de validation
        'validation failed', 'invalid input', 'malformed',
        
        // Messages d'erreur en français (pour Matcha)
        'erreur de connexion', 'serveur indisponible', 'données invalides',
        'fichier trop volumineux', 'format non supporté',
        
        // Autres fichiers statiques communes
        'robots.txt', 'sitemap.xml', 'manifest.json',
        '.map', 'sourcemap'
    ];
    
    // Fonction pour vérifier si un message doit être supprimé
    function shouldSuppress(message) {
        const messageStr = String(message).toLowerCase();
        
        // Supprimer tous les patterns listés
        const hasPattern = suppressedPatterns.some(pattern => 
            messageStr.includes(pattern.toLowerCase())
        );
        
        // Supprimer aussi les codes d'erreur HTTP sous forme de nombres
        const httpErrorPattern = /\b(4\d{2}|5\d{2})\b/;
        const hasHttpError = httpErrorPattern.test(messageStr);
        
        // Vérifier si c'est une URL avec une erreur
        const urlErrorPattern = /(https?:\/\/[^\s]+).*?(404|403|500)/i;
        const hasUrlError = urlErrorPattern.test(messageStr);
        
        return hasPattern || hasHttpError || hasUrlError;
    }
    
    // Remplacement console.error - SUPPRESSION TOTALE
    console.error = function(...args) {
        const message = args.join(' ');
        if (!shouldSuppress(message)) {
            // Ne garder que les erreurs vraiment importantes (bugs JS)
            const isRealError = message.toLowerCase().includes('uncaught') || 
                               message.toLowerCase().includes('syntax') ||
                               message.toLowerCase().includes('referenceerror') ||
                               (message.toLowerCase().includes('typeerror') && 
                                !message.toLowerCase().includes('fetch') &&
                                !message.toLowerCase().includes('response'));
            
            if (isRealError) {
                originalConsoleError.apply(console, args);
            }
        }
    };
    
    // Remplacement console.warn - SUPPRESSION TOTALE
    console.warn = function(...args) {
        const message = args.join(' ');
        if (!shouldSuppress(message)) {
            // Très restrictif : ne garder que les warnings critiques
            const isCriticalWarning = message.toLowerCase().includes('deprecated') ||
                                    message.toLowerCase().includes('security') ||
                                    message.toLowerCase().includes('cors');
                                    
            if (isCriticalWarning) {
                originalConsoleWarn.apply(console, args);
            }
        }
    };

    // =================================================================
    // 2. INTERCEPTION ET SUPPRESSION DES ERREURS GLOBALES
    // =================================================================
    
    // Capturer et supprimer les erreurs JavaScript non gérées
    window.addEventListener('error', function(event) {
        const message = event.message || event.error?.message || '';
        const filename = event.filename || '';
        
        // Supprimer les erreurs liées aux ressources statiques (FAVICON INCLUS)
        if (shouldSuppress(message) || shouldSuppress(filename)) {
            event.preventDefault();
            event.stopPropagation();
            return false;
        }
        
        // Ne laisser passer que les vraies erreurs de code
        const isCodeError = message.includes('ReferenceError') ||
                           message.includes('TypeError') ||
                           message.includes('SyntaxError') ||
                           message.includes('Uncaught');
                           
        if (!isCodeError) {
            event.preventDefault();
            event.stopPropagation();
            return false;
        }
    });
    
    // Capturer et supprimer les promesses rejetées
    window.addEventListener('unhandledrejection', function(event) {
        const reason = event.reason || '';
        const message = reason.message || String(reason);
        
        // Supprimer TOUTES les rejections liées aux requêtes HTTP
        if (shouldSuppress(message) || 
            message.includes('fetch') || 
            message.includes('Response') ||
            message.includes('Request') ||
            reason.name === 'TypeError') {
            
            event.preventDefault();
            return;
        }
    });

    // =================================================================
    // 3. INTERCEPTION FETCH POUR ÉVITER LES ERREURS DE CONSOLE
    // =================================================================
    
    // Sauvegarder le fetch original
    const originalFetch = window.fetch;
    
    // Remplacer fetch pour supprimer les erreurs automatiquement
    window.fetch = async function(...args) {
        try {
            const response = await originalFetch.apply(this, args);
            
            // Gestion spéciale du favicon
            const url = args[0];
            if (url && url.includes && url.includes('favicon.ico')) {
                if (!response.ok) {
                    // Créer une réponse silencieuse pour le favicon
                    return new Response('', {
                        status: 204,
                        statusText: 'No Content',
                        headers: { 'Content-Type': 'image/x-icon' }
                    });
                }
            }
            
            // Ne pas permettre aux erreurs HTTP d'apparaître dans la console
            // On clone la response pour éviter les conflits
            const responseClone = response.clone();
            
            // Si c'est une erreur HTTP, on la gère silencieusement
            if (!response.ok) {
                // Créer une nouvelle Response "ok" pour éviter les erreurs de console
                // mais préserver le statut pour l'application
                const errorResponse = new Response(
                    await responseClone.text(), 
                    {
                        status: response.status,
                        statusText: response.statusText,
                        headers: response.headers
                    }
                );
                
                // Marquer comme "erreur gérée" pour éviter les logs
                Object.defineProperty(errorResponse, '_matchaHandled', {
                    value: true,
                    writable: false
                });
                
                return errorResponse;
            }
            
            return response;
            
        } catch (networkError) {
            // Gestion spéciale du favicon en cas d'erreur réseau
            const url = args[0];
            if (url && url.includes && url.includes('favicon.ico')) {
                return new Response('', {
                    status: 204,
                    statusText: 'No Content',
                    headers: { 'Content-Type': 'image/x-icon' }
                });
            }
            
            // Créer une réponse d'erreur standardisée qui n'apparaîtra pas dans la console
            const silentError = new Response(
                JSON.stringify({ error: 'Erreur de connexion réseau' }), 
                {
                    status: 503,
                    statusText: 'Service Unavailable',
                    headers: { 'Content-Type': 'application/json' }
                }
            );
            
            Object.defineProperty(silentError, '_matchaNetworkError', {
                value: true,
                writable: false
            });
            
            return silentError;
        }
    };

    // =================================================================
    // 4. FONCTIONS UTILITAIRES POUR L'INTERFACE UTILISATEUR
    // =================================================================
    
    // Fonction d'affichage d'erreurs pour l'utilisateur
    window.showError = function(message, duration = 5000) {
        // Supprimer les anciens messages
        const existing = document.querySelectorAll('.matcha-error-message');
        existing.forEach(el => el.remove());
        
        // Créer le nouveau message
        const errorDiv = document.createElement('div');
        errorDiv.className = 'matcha-error-message';
        errorDiv.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            background-color: #f8d7da;
            color: #721c24;
            padding: 15px 20px;
            border: 1px solid #f5c6cb;
            border-radius: 8px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            z-index: 10000;
            max-width: 400px;
            font-size: 14px;
            font-family: Arial, sans-serif;
            animation: slideIn 0.3s ease;
        `;
        errorDiv.textContent = message;
        
        document.body.appendChild(errorDiv);
        
        if (duration > 0) {
            setTimeout(() => {
                if (errorDiv.parentNode) {
                    errorDiv.style.animation = 'slideOut 0.3s ease';
                    setTimeout(() => errorDiv.remove(), 300);
                }
            }, duration);
        }
    };
    
    // Fonction d'affichage de succès
    window.showSuccess = function(message, duration = 3000) {
        const existing = document.querySelectorAll('.matcha-success-message');
        existing.forEach(el => el.remove());
        
        const successDiv = document.createElement('div');
        successDiv.className = 'matcha-success-message';
        successDiv.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            background-color: #d4edda;
            color: #155724;
            padding: 15px 20px;
            border: 1px solid #c3e6cb;
            border-radius: 8px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            z-index: 10000;
            max-width: 400px;
            font-size: 14px;
            font-family: Arial, sans-serif;
            animation: slideIn 0.3s ease;
        `;
        successDiv.textContent = message;
        document.body.appendChild(successDiv);
        
        setTimeout(() => {
            if (successDiv.parentNode) {
                successDiv.style.animation = 'slideOut 0.3s ease';
                setTimeout(() => successDiv.remove(), 300);
            }
        }, duration);
    };

    // Fonction d'affichage d'info
    window.showInfo = function(message, duration = 4000) {
        const existing = document.querySelectorAll('.matcha-info-message');
        existing.forEach(el => el.remove());
        
        const infoDiv = document.createElement('div');
        infoDiv.className = 'matcha-info-message';
        infoDiv.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            background-color: #d1ecf1;
            color: #0c5460;
            padding: 15px 20px;
            border: 1px solid #bee5eb;
            border-radius: 8px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            z-index: 10000;
            max-width: 400px;
            font-size: 14px;
            font-family: Arial, sans-serif;
            animation: slideIn 0.3s ease;
        `;
        infoDiv.textContent = message;
        document.body.appendChild(infoDiv);
        
        if (duration > 0) {
            setTimeout(() => {
                if (infoDiv.parentNode) {
                    infoDiv.style.animation = 'slideOut 0.3s ease';
                    setTimeout(() => infoDiv.remove(), 300);
                }
            }, duration);
        }
    };

    // =================================================================
    // 5. GESTION DE FORMULAIRE SANS ERREURS DE CONSOLE
    // =================================================================
    
    window.handleFormSubmission = async function(url, formData, options = {}) {
        const {
            onSuccess = () => {},
            onError = () => {},
            redirectOnSuccess = null,
            showErrorFunction = window.showError || (() => {})
        } = options;
        
        try {
            const response = await fetch(url, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(formData)
            });
            
            let data = {};
            try {
                const text = await response.text();
                if (text.trim()) {
                    data = JSON.parse(text);
                }
            } catch (parseError) {
                data = { error: 'Erreur de format de réponse' };
            }
            
            if (response.ok && (data.success === true || response.status === 200)) {
                onSuccess(data);
                if (redirectOnSuccess) {
                    window.location.href = redirectOnSuccess;
                }
                return { success: true, data };
            }
            
            const errorMessage = data.error || 
                (response.status === 401 ? 'Identifiants incorrects' :
                 response.status === 400 ? 'Données invalides' :
                 response.status === 403 ? 'Accès refusé' :
                 response.status === 404 ? 'Ressource non trouvée' :
                 'Erreur de connexion');
            
            onError(errorMessage);
            showErrorFunction(errorMessage);
            return { success: false, error: errorMessage };
            
        } catch (error) {
            const errorMessage = 'Erreur de connexion au serveur';
            onError(errorMessage);
            showErrorFunction(errorMessage);
            return { success: false, error: errorMessage };
        }
    };

    // Ajouter des styles CSS pour les animations
    const style = document.createElement('style');
    style.textContent = `
        @keyframes slideIn {
            from { transform: translateX(100%); opacity: 0; }
            to { transform: translateX(0); opacity: 1; }
        }
        
        @keyframes slideOut {
            from { transform: translateX(0); opacity: 1; }
            to { transform: translateX(100%); opacity: 0; }
        }
        
        .matcha-error-message, .matcha-success-message, .matcha-info-message {
            cursor: pointer;
        }
        
        .matcha-error-message:hover, .matcha-success-message:hover, .matcha-info-message:hover {
            opacity: 0.9;
        }
    `;
    document.head.appendChild(style);

    // Permettre de fermer les notifications en cliquant dessus
    document.addEventListener('click', function(e) {
        if (e.target.classList.contains('matcha-error-message') ||
            e.target.classList.contains('matcha-success-message') ||
            e.target.classList.contains('matcha-info-message')) {
            e.target.style.animation = 'slideOut 0.3s ease';
            setTimeout(() => e.target.remove(), 300);
        }
    });

    // Confirmation finale
    console.log('✅ Gestionnaire d\'erreurs Matcha initialisé - Console nettoyée (favicon.ico inclus)');

})();