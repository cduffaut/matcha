// Variables globales
let messages = [];
let currentConversationUser = null;
let messageOffset = 0;
let isLoadingMessages = false;
let isMobileView = false;
let pollingInterval = null;
let lastMessageCount = 0;

// Initialisation au chargement de la page
document.addEventListener('DOMContentLoaded', function() {
    
    // Détecter mobile dès le début
    detectMobile();
    
    // ✅ INITIALISER L'ID UTILISATEUR EN PREMIER
    setCurrentUserId().then(() => {
        loadConversations();
        setupEventListeners();
        loadUnreadCounts();
        
        // Initialiser le mode mobile si nécessaire
        if (isMobileView) {
            initMobileChat();
        }
        
        // Vérifier s'il y a un utilisateur spécifique à ouvrir
        const urlParams = new URLSearchParams(window.location.search);
        const userParam = urlParams.get('user');
        if (userParam) {
            const userId = parseInt(userParam);
            if (!isNaN(userId)) {
                setTimeout(() => {
                    openConversation(userId, 'Utilisateur');
                }, 500);
            }
        }
    });
});

// ✅ INITIALISER L'ID UTILISATEUR
async function setCurrentUserId() {
    // Si l'ID est déjà injecté par le serveur, l'utiliser
    if (window.currentUserId) {
        return window.currentUserId;
    }
    
    // Fallback: récupérer depuis l'API
    try {
        const response = await fetch('/api/profile');
        if (response.ok) {
            const profile = await response.json();
            window.currentUserId = profile.UserID || profile.user_id || profile.id;
            return window.currentUserId;
        } else {
            return null;
        }
    } catch (error) {
        return null;
    }
}

// Détection mobile améliorée - CORRECTION DE LA FAUTE DE FRAPPE
function detectMobile() {
    const width = window.innerWidth;
    const isTouchDevice = 'ontouchstart' in window || navigator.maxTouchPoints > 0;
    isMobileView = width <= 768;
    
    // Ajouter classe CSS pour cibler spécifiquement
    document.body.classList.toggle('mobile-view', isMobileView);
    document.body.classList.toggle('touch-device', isTouchDevice);
    
    return isMobileView;
}

// Fonction pour détecter si on est sur mobile (alias)
function isMobile() {
    return isMobileView || window.innerWidth <= 768;
}

// Initialisation mobile
function initMobileChat() {
    if (!isMobileView) return;
    
    
    // Reset initial - montrer la liste des conversations
    showConversationsList();
    
    // Ajouter les event listeners pour mobile
    addMobileEventListeners();
    
    // Configurer le viewport pour mobile
    setupMobileViewport();
    
    // Optimiser les inputs
    setTimeout(optimizeMobileInput, 100);
}

// Configuration du viewport mobile
function setupMobileViewport() {
    let viewport = document.querySelector('meta[name=viewport]');
    if (!viewport) {
        viewport = document.createElement('meta');
        viewport.name = 'viewport';
        document.head.appendChild(viewport);
    }
    viewport.content = 'width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no';
}

// Gestion des événements tactiles
function addMobileEventListeners() {
    // Écouteur pour le redimensionnement
    window.addEventListener('resize', () => {
        const wasMobile = isMobileView;
        detectMobile();
        handleLayoutChange(wasMobile);
    });
    
    // Écouteur pour l'orientation
    window.addEventListener('orientationchange', () => {
        setTimeout(handleOrientationChange, 100);
    });
    
    // Prévenir le zoom sur double-tap
    document.addEventListener('touchend', (e) => {
        const now = Date.now();
        if (now - (window.lastTouchEnd || 0) < 300) {
            e.preventDefault();
        }
        window.lastTouchEnd = now;
    });
}

// Gérer le changement de layout - CORRECTION DE LA VARIABLE
function handleLayoutChange(wasMobile) {
    // Si on passe de mobile à desktop ou vice versa
    if (wasMobile !== isMobileView) {
        
        if (isMobileView) {
            // Passage en mobile
            showConversationsList();
        } else {
            // Passage en desktop
            const conversationsList = document.querySelector('.conversations-list');
            const chatArea = document.querySelector('.chat-area');
            
            if (conversationsList && chatArea) {
                conversationsList.classList.remove('show');
                conversationsList.style.display = 'flex';
                chatArea.style.display = 'flex';
            }
        }
    }
}

// Afficher la liste des conversations (mobile)
function showConversationsList() {
    if (!isMobileView) return;
    
    // 🆕 ARRÊTER LE POLLING quand on quitte la conversation
    stopMessagePolling();
    
    const conversationsList = document.querySelector('.conversations-list');
    const chatArea = document.querySelector('.chat-area');
    
    if (conversationsList && chatArea) {
        // Montrer la liste
        conversationsList.classList.add('show');
        
        // Cacher la zone de chat
        chatArea.style.display = 'none';
        
        // Reset du header
        resetChatHeader();
        
        // Reset de l'état actuel
        currentConversationUser = null;
        
        // Cacher l'input de message
        const inputContainer = document.getElementById('message-input-container');
        if (inputContainer) {
            inputContainer.style.display = 'none';
        }
    }
}

// Afficher la zone de chat (mobile)
function showChatArea(userId, userName) {
    if (!isMobileView) return;
    
    
    const conversationsList = document.querySelector('.conversations-list');
    const chatArea = document.querySelector('.chat-area');
    
    if (conversationsList && chatArea) {
        // Cacher la liste
        conversationsList.classList.remove('show');
        
        // Montrer la zone de chat
        chatArea.style.display = 'flex';
        
        // Mettre à jour le header
        updateChatHeader(userName);
        
        // Stocker l'utilisateur actuel
        currentConversationUser = userId;
        
        // Montrer l'input de message
        const inputContainer = document.getElementById('message-input-container');
        if (inputContainer) {
            inputContainer.style.display = 'block';
        }
        
        // Faire défiler vers le bas
        setTimeout(scrollToBottom, 100);
    }
}

// Mettre à jour le header du chat
function updateChatHeader(userName) {
    const chatHeader = document.getElementById('chat-header') || document.querySelector('.chat-header');
    
    if (chatHeader) {
        chatHeader.innerHTML = `
            <button class="back-button" onclick="handleBackButton()">←</button>
            <span>${userName || 'Chat'}</span>
        `;
    }
}

// Reset du header du chat
function resetChatHeader() {
    const chatHeader = document.getElementById('chat-header') || document.querySelector('.chat-header');
    
    if (chatHeader) {
        if (isMobileView) {
            chatHeader.innerHTML = `
                <button class="back-button" onclick="handleBackButton()">←</button>
                <span>Sélectionnez une conversation</span>
            `;
        } else {
            chatHeader.textContent = 'Sélectionnez une conversation';
        }
    }
}

// Gérer le bouton retour
function handleBackButton() {
    
    if (isMobileView) {
        showConversationsList();
    }
}

// Charger les conversations
async function loadConversations() {
    try {
        const response = await fetch('/api/chat/conversations');
        if (response.ok) {
            const conversations = await response.json();
            displayConversations(conversations);
        } else {
            return null;
        }
    } catch (error) {
        return null;
    }
}

// Afficher la liste des conversations
function displayConversations(conversations) {
    const container = document.getElementById('conversations');
    if (!container) return;
    
    container.innerHTML = '';
    
    if (!conversations || conversations.length === 0) {
        container.innerHTML = '<div class="no-conversations">Aucune conversation</div>';
        return;
    }
    
    conversations.forEach(conversation => {
        const conversationElement = createConversationElement(conversation);
        container.appendChild(conversationElement);
    });
}

// Créer un élément de conversation - AVEC SUPPORT MOBILE
function createConversationElement(conversation) {
    const div = document.createElement('div');
    div.className = 'conversation-item';
    div.dataset.userId = conversation.user_id;
    div.dataset.userName = conversation.name || conversation.username || 'Utilisateur';
    
    const isNewMatch = !conversation.last_message;
    
    let lastMessageDisplay = '';
    if (conversation.last_message) {
        const preview = conversation.last_message.content.length > 50 ? 
            conversation.last_message.content.substring(0, 50) + '...' 
            : conversation.last_message.content;
        lastMessageDisplay = `<div class="last-message">${preview}</div>`;
    }
    
    div.innerHTML = `
        <div class="conversation-info">
            <div class="conversation-name">${conversation.name || conversation.username}</div>
            ${lastMessageDisplay}
        </div>
        <div class="conversation-meta">
            ${isNewMatch ? '<div class="match-badge">MATCH</div>' : ''}
            ${conversation.unread_count > 0 ? 
                `<div class="unread-badge">${conversation.unread_count}</div>` : ''}
        </div>
    `;
    
    div.addEventListener('click', () => {
        const userName = conversation.name || conversation.username || 'Utilisateur';
        handleConversationClick(conversation.user_id, userName);
    });
    
    return div;
}

// Gérer le clic sur une conversation - FONCTION CORRIGÉE
function handleConversationClick(userId, userName) {
    
    if (isMobileView) {
        showChatArea(userId, userName);
    }
    
    // Ouvrir la conversation (logique existante)
    openConversation(userId, userName);
}

// Ouvrir une conversation
async function openConversation(userId, userName) {
    
    currentConversationUser = userId;
    messageOffset = 0;
    messages = [];
    
    // Marquer la conversation comme active
    document.querySelectorAll('.conversation-item').forEach(item => {
        item.classList.remove('active');
    });
    
    const activeConv = document.querySelector(`[data-user-id="${userId}"]`);
    if (activeConv) {
        activeConv.classList.add('active');
    }
    
    // Mettre à jour l'en-tête
    const headerElement = document.getElementById('chat-header');
    if (headerElement) {
        if (isMobileView) {
            updateChatHeader(userName);
        } else {
            headerElement.textContent = userName;
        }
    }
    
    // Afficher la zone de saisie
    const inputContainer = document.getElementById('message-input-container');
    if (inputContainer) {
        inputContainer.style.display = 'block';
    }
    
    // Charger les messages
    await loadMessages();
    lastMessageCount = messages.length; // 🆕 NOUVEAU
    
    // 🆕 DÉMARRER LE POLLING pour cette conversation
    startMessagePolling();
    
    // Marquer les messages comme lus
    await markAsRead(userId);
    
    // Nettoyer l'URL
    if (window.location.search.includes('user=')) {
        const newURL = window.location.pathname;
        window.history.replaceState({}, document.title, newURL);
    }
}

// ✅ CHARGER LES MESSAGES (VERSION CORRIGÉE)
async function loadMessages() {
    if (isLoadingMessages || !currentConversationUser) {
        return;
    }
    
    isLoadingMessages = true;
    
    try {
        const response = await fetch(`/api/chat/conversation/${currentConversationUser}?limit=50&offset=${messageOffset}`);
        
        if (!response.ok) {
            if (response.status === 400 && messageOffset === 0) {
                displayNoMatchMessage();
            }
            return;
        }
        
        // ✅ GESTION STRICTE DE LA RÉPONSE
        let responseData;
        const contentType = response.headers.get('content-type');
        
        if (contentType && contentType.includes('application/json')) {
            responseData = await response.json();
        } else {
            if (messageOffset === 0) {
                displayNoMatchMessage();
            }
            return;
        }
        
        // ✅ VÉRIFICATION STRICTE DU FORMAT
        if (!responseData || !Array.isArray(responseData)) {
            if (messageOffset === 0) {
                displayNoMatchMessage();
            }
            return;
        }
        
        const newMessages = responseData;
        
        if (messageOffset === 0) {
            messages = [...newMessages];
        } else {
            // ✅ ÉVITER LES DOUBLONS lors du chargement de pages supplémentaires
            const existingIds = new Set(messages.map(m => m.id));
            const uniqueNewMessages = newMessages.filter(m => m && m.id && !existingIds.has(m.id));
            messages = [...uniqueNewMessages, ...messages];
        }
        
        messageOffset += newMessages.length;
        displayMessages();
        
        if (messageOffset === newMessages.length) {
            scrollToBottom();
        }
        
    } catch (error) {
        if (messageOffset === 0) {
            const container = document.getElementById('messages-container');
            if (container) {
                container.innerHTML = '<div class="no-conversation"><p>Erreur de chargement des messages</p></div>';
            }
        }
    } finally {
        isLoadingMessages = false;
    }
}

// Afficher un message pour les nouveaux matchs
function displayNoMatchMessage() {
    const container = document.getElementById('messages-container');
    if (container) {
        container.innerHTML = `
            <div class="no-conversation">
                <div class="welcome-message">
                    <h3>🎉 Nouveau match !</h3>
                    <p>Vous pouvez maintenant discuter avec cette personne.</p>
                    <p>Commencez la conversation en envoyant un message ci-dessous.</p>
                </div>
            </div>
        `;
    }
}

// ✅ AFFICHER LES MESSAGES (VERSION NETTOYÉE)
function displayMessages() {
    const container = document.getElementById('messages-container');
    if (!container) return;
    
    container.innerHTML = '';
    
    if (!Array.isArray(messages) || messages.length === 0) {
        container.innerHTML = '<div class="no-conversation"><p>Aucun message dans cette conversation</p></div>';
        return;
    }
    
    // ✅ ÉLIMINER LES DOUBLONS PAR ID avec validation
    const uniqueMessages = [];
    const seenIds = new Set();
    
    messages.forEach(message => {
        if (message && message.id && !seenIds.has(message.id)) {
            seenIds.add(message.id);
            uniqueMessages.push(message);
        }
    });
    
    // Trier par date de création
    uniqueMessages.sort((a, b) => {
        const dateA = new Date(a.created_at);
        const dateB = new Date(b.created_at);
        return dateA - dateB;
    });
    
    uniqueMessages.forEach(message => {
        const messageElement = createMessageElement(message);
        container.appendChild(messageElement);
    });
    
}

// Créer un élément de message
function createMessageElement(message) {
    const div = document.createElement('div');
    const currentUserId = getCurrentUserId();
    const isCurrentUser = message.sender_id === currentUserId;
    
    div.className = `message ${isCurrentUser ? 'sent' : 'received'}`;
    div.dataset.messageId = message.id;
    
    let senderInfo = '';
    if (!isCurrentUser && message.sender_username) {
        senderInfo = `<div class="message-sender">${message.sender_name || message.sender_username}</div>`;
    }
    
    div.innerHTML = `
        ${senderInfo}
        <div class="message-content">${escapeHtml(message.content)}</div>
        <div class="message-time">${formatMessageTime(message.created_at)}</div>
    `;
    
    return div;
}

async function sendMessage(e) {
    e.preventDefault();
    
    if (!currentConversationUser) {
        alert('Sélectionnez une conversation');
        return;
    }
    
    const messageInput = document.getElementById('message-input');
    if (!messageInput) {
        return;
    }
    
    const content = messageInput.value.trim();
    if (!content) return;
    
    const submitButton = document.querySelector('#message-form button[type="submit"]');
    if (submitButton) submitButton.disabled = true;
    messageInput.disabled = true;
    
    try {
        const requestData = {
            recipient_id: parseInt(currentConversationUser),
            content: content
        };
        
        const response = await fetch('/api/chat/send', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(requestData)
        });
        
        if (response.ok) {
            const sentMessage = await response.json();
            
            // Vider le champ
            messageInput.value = '';
            
            // Recharger les messages
            messageOffset = 0;
            await loadMessages();
            
            displayMessages();
            
            // Scroll vers le bas
            scrollToBottom();
            
        } else {
            const errorText = await response.text();
            alert('Erreur lors de l\'envoi: ' + errorText);
        }
    } catch (error) {
        alert('Erreur de connexion');
    } finally {
        if (submitButton) submitButton.disabled = false;
        messageInput.disabled = false;
        messageInput.focus();
    }
}

// ✅ CONFIGURATION DES EVENT LISTENERS (VERSION SÉCURISÉE)
function setupEventListeners() {
    
    const messageForm = document.getElementById('message-form');
    
    if (messageForm) {
        // ✅ Retirer les anciens listeners pour éviter les doublons
        const newForm = messageForm.cloneNode(true);
        messageForm.parentNode.replaceChild(newForm, messageForm);
        
        newForm.addEventListener('submit', sendMessage);
        
        const newInput = document.getElementById('message-input');
        if (newInput) {
            newInput.addEventListener('keypress', function(e) {
                if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    sendMessage(e);
                }
            });
        }
    }
}

// Optimiser l'input pour mobile
function optimizeMobileInput() {
    const messageInput = document.getElementById('message-input');
    
    if (messageInput && isMobileView) {
        // Éviter le zoom sur iOS
        messageInput.style.fontSize = '16px';
        
        // Gérer le focus/blur pour ajuster la vue
        messageInput.addEventListener('focus', () => {
            // Petit délai pour laisser le clavier s'ouvrir
            setTimeout(() => {
                messageInput.scrollIntoView({ 
                    behavior: 'smooth', 
                    block: 'center' 
                });
            }, 300);
        });
        
        messageInput.addEventListener('blur', () => {
            // Rescroller vers le bas quand le clavier se ferme
            setTimeout(scrollToBottom, 300);
        });
    }
}

// Gérer le changement d'orientation
function handleOrientationChange() {
    if (!isMobileView) return;
    
    
    // Recalculer les hauteurs
    setTimeout(() => {
        const messagesContainer = document.querySelector('.messages-container');
        if (messagesContainer) {
            messagesContainer.style.height = 'auto';
            setTimeout(() => {
                messagesContainer.style.height = '';
                scrollToBottom();
            }, 50);
        }
    }, 300);
}

// Marquer les messages comme lus
async function markAsRead(userId) {
    try {
        await fetch(`/api/chat/conversation/${userId}/read`, {
            method: 'PUT'
        });
        
        const convElement = document.querySelector(`[data-user-id="${userId}"]`);
        if (convElement) {
            const badge = convElement.querySelector('.unread-badge');
            if (badge) {
                badge.remove();
            }
        }
        
        loadUnreadCounts();
    } catch (error) {
        return null;
    }
}

// Charger les compteurs
async function loadUnreadCounts() {
    try {
        const messagesResponse = await fetch('/api/chat/unread-count');
        if (messagesResponse.ok) {
            const data = await messagesResponse.json();
            updateMessageCount(data.unread_count);
        }
        
        const notificationsResponse = await fetch('/api/notifications/unread-count');
        if (notificationsResponse.ok) {
            const data = await notificationsResponse.json();
            updateNotificationCount(data.unread_count);
        }
    } catch (error) {
        return null;
    }
}

// Mettre à jour les compteurs
function updateMessageCount(count) {
    const countElement = document.getElementById('message-count');
    if (countElement) {
        if (count > 0) {
            countElement.textContent = count;
            countElement.style.display = 'inline';
        } else {
            countElement.style.display = 'none';
        }
    }
}

function updateNotificationCount(count) {
    const countElement = document.getElementById('notification-count');
    if (countElement) {
        if (count > 0) {
            countElement.textContent = count;
            countElement.style.display = 'inline';
        } else {
            countElement.style.display = 'none';
        }
    }
}

// fonction qui gère correctement l'UTC
function formatMessageTime(timestamp) {
    if (!timestamp) return '';
    
    const messageDate = new Date(timestamp);
    if (isNaN(messageDate.getTime())) return 'Date invalide';
    
    // Maintenant
    const now = new Date();
    
    // Si le message a été envoyé dans les 2 dernières minutes
    const diffMs = now - messageDate;
    const diffMinutes = Math.floor(diffMs / (1000 * 60));
    
    if (diffMinutes >= -2 && diffMinutes <= 2) {
        return 'À l\'instant';
    }
    
    // Sinon afficher l'heure locale normalement
    return messageDate.toLocaleTimeString('fr-FR', { 
        hour: '2-digit', 
        minute: '2-digit' 
    });
}

// Faire défiler vers le bas
function scrollToBottom() {
    const messagesContainer = document.getElementById('messages-container') || 
                            document.querySelector('.messages-container');
    
    if (messagesContainer) {
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
    }
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text.toString();
    return div.innerHTML;
}

// ✅ FONCTION getCurrentUserId SIMPLIFIÉE
function getCurrentUserId() {
    if (window.currentUserId) {
        return parseInt(window.currentUserId);
    }
    
    return null;
}

// Fonction pour démarrer le polling automatique
function startMessagePolling() {
    // Arrêter le polling précédent s'il existe
    if (pollingInterval) {
        clearInterval(pollingInterval);
    }
    
    // Vérifier les nouveaux messages toutes les 3 secondes
    pollingInterval = setInterval(async () => {
        if (currentConversationUser) {
            await checkForNewMessages();
        }
    }, 3000); // 3 secondes
}

// Fonction pour arrêter le polling
function stopMessagePolling() {
    if (pollingInterval) {
        clearInterval(pollingInterval);
        pollingInterval = null;
    }
}

// Vérifier s'il y a de nouveaux messages
async function checkForNewMessages() {
    if (!currentConversationUser || isLoadingMessages) {
        return;
    }
    
    try {
        const response = await fetch(`/api/chat/conversation/${currentConversationUser}?limit=50&offset=0`);
        
        if (response.ok) {
            const newMessages = await response.json();
            
            // Si on a plus de messages qu'avant, recharger l'affichage
            if (newMessages.length > lastMessageCount) {
                
                // Mettre à jour les messages
                messages = [...newMessages];
                lastMessageCount = newMessages.length;
                
                // Rafraîchir l'affichage
                displayMessages();
                scrollToBottom();
                
                // Marquer comme lu
                await markAsRead(currentConversationUser);
            } else {
                lastMessageCount = newMessages.length;
            }
        }
    } catch (error) {
        return null;
    }
}

// Nettoyer le polling quand on quitte la page
window.addEventListener('beforeunload', () => {
    stopMessagePolling();
});

// Utilisation externe des fonctions
window.mobileChat = {
    showConversationsList,
    showChatArea,
    handleConversationClick,
    handleBackButton,
    scrollToBottom,
    detectMobile,
};