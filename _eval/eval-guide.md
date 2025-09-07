# Eval requirements

<details>
<summary>Security</summary>

```md
The subject insisted on that point: the website must be secured.
Check at least the following points:  
- Password are encripted in the database.
- Forms and uploads have correct validations.
- SQL injection isn't possible.
```

1. Mots de passes chiffrés dans la database  

```bash
# acceder au conteneur de la DB avec shell psql
docker exec -it matcha-db psql -U <DB_USER> -d <DB_NAME>
```

```sql
--- afficher tout les champs des 5 derniers utilisateurs inscrits
SELECT * FROM users ORDER BY id DESC LIMIT 5;
```

**Si les mots de passes sont en clair, failed**

2. Formulaires et uploads secures
```bash
# A tester dans l'UI
```

3. Injection SQL

A entrer dans un champ de formulaire
```sql
' OR '1'='1
```
```sql
' OR '1'='1' --
```
```sql
'; DROP TABLE users; --
```
**Si requete reussie, failed**

### Resultat - ⏳ a corriger

Protection injection SQL :  
- Biographie de l'user  
- Tags dans la page Explorer  

</details>

<br>

---

<br>

<details>
<summary>Installation and seeding</summary>

```md
Re-do the whole installation of every package with the evaluee.
You must also fill the database with the script he wrote.
Make sure that the database at least contains 500 different profiles.
```

1. Tout couper, et tout relancer : 
```bash
docker compose down # stop tout les conteneurs
docker system prune -af # supprime tout les conteneurs et volumes
docker compose up --build -d # build de l'ensemble
```

2. Verifier les logs des conteneurs :  
```bash
docker logs -f matcha-app
docker logs -f matcha-db
```

3. Lister combien d'utilisateurs sont inscrits :  

```bash
# acceder au conteneur de la DB avec shell psql
docker exec -it matcha-db psql -U <DB_USER> -d <DB_NAME>
```

```sql
-- liste tout les entrees de la table 'users'
SELECT COUNT(*) FROM users;
```

**Si count < 500, failed**

4. Fichiers relatifs au remplissage des 500 donnees aleatoires (facultatif) :  

- [Script SQL pour migration](./internal/database/migrations/add_500_seed.sql)  
- [Fichiers CSV mock data](./mock)  
- [Migrations DB Backend](./internal/database/database.go)  

### Resultat - ✅ OK

</details>

<br>

---

<br>

<details>
<summary>Features</summary>

```md
During the defense, keep the web console open at all times. If at least one error, notice, or warning appears, select "Crash" at the very bottom of the checklist. An error code from 500 to 599 returned by the server is also considered to be a Crash.
Simple start
Launch the webserver containing the website.
No errors must be visible.
```

1. Pas de logs d'erreurs dans la console
```bash
# touche F12 (ou clic droit inspecter), verification continue sur le browser
```

**Si erreur non geree, failed**

### Resultat - 

Login incorrect :  
- Erreur 401 affichee dans le terminal 

</details>

<br>

---

<br>

<details>
<summary>User account management</summary>

```md
The app must allow a user to register asking at least an email address, a username, a last name, a first name and a password that is somehow protected. (An english common word shouldn't be accepted for example.)
A connected user must be able to fill an extended profile, and must be able to update his information as well as the one given during registration, at any time.
When you subscribe, you are emailed a clickable link.
If you haven't clicked the link, the account must not be usable.
```

1. Mail + compte inaccessible tant que non valide 
```bash
# test dans l'UI (page login)
```

2. Politique de mot de passe  

A verifier ici --> [validation.go](./internal/validation/validation.go) 

### Resultat - ✅ OK

</details>

<br>

---

<br>

<details>
<summary>User connexion</summary>

```md
The user must then be able to connect with his username and password. He must be able to receive an email allowing him to re-initialize his password should the first one be forgotten.
To disconnect must be possible from any page on the site
with one click.
```

1. Connexion via user/mdp  
```bash
# a tester dans l'UI (page login)
```

2. Reinit du mot de passe (reception par mail + fallback logs)  
```bash
# a tester dans l'UI (page ' Mot de passe oublie ')
```

3. Logout a partir de toutes les pages  
```bash
# a tester dans l'UI (tout les pages)
```

### Resultat - ✅ OK


</details>

<br>

---

<br>

<details>
<summary>Extended profile</summary>

```md
The user must be able to fill in the following:
- His sex 
- His sexual orientation
- Short bio
- Interests list (with hashtags \#bio, \#NoMakeup...)
- Images, up to 5, including a profile picture
If the seed is correctly implemented, you can make tag propositions in any form you want (autocomplete, top-trending)
Once his profile is complete, he can access the website.
These informations can be changed at any time, once connected.
If one of the points fails, this question is false
```

1. Sexe, orientation sexuelle, bio, tag/interets, photos dont photo principale
```bash
# a tester dans l'UI (page Mon Profil)
```

2. Profil etendu a configurer, autres membres inaccessibles tant que pas complet  
```bash
# a tester dans l'UI (page Explorer)
```

3. Suggestions de tags
```bash
# a tester dans l'UI (page Mon Profil)
```

### Resultat - ⏳ a corriger

Suggestions de tags :  
- dropdown de tags deja existants OU suggestion selon les seeds    

</details>

<br>

---

<br>

<details>
<summary>Consultations</summary>

```md
The user must be able to check out the people that looked at his profile (there mush be an history of visits) as well as the people that "liked" him.
```

1. Notification quand visite + historique
```bash
# a tester dans l'UI :
# 	- page Explorer pour visite
# 	- page Mon Profil pour historique (lien 'Voir qui a visite mon profil')
# 	- page Mon Profil pour historique (lien 'Voir qui m'a like')
#	- page Notifications
```

### Resultat - ✅ OK

</details>

<br>

---


<br>

<details>
<summary>Fame rating</summary>

```md
Each user must have a public fame rating. Ask the student to explain his stategy regarding the computing of that score, it must be consistent and a minimum relevant.
```

1. Explication calcul du Fame Rating (cduffaut)  
```bash
# basé sur le nombre de visites, de likes et de matchs
# formule : visites = 1 point, likes = 2 points, matchs = 5 points
```
[Voir dans le code](./internal/user/profile_repository.go) -> fonction `UpdateFameRating`

2. Voir Fame Rating
```bash
# a tester dans l'UI :
#	- page Mon Profil (voir son propre score)
#	- page Explorer (voir score des autres)
```

### Resultat - ✅ OK

</details>

<br>

---

<br>

<details>
<summary>Geolocalisation</summary>

```md
The user must be located using GPS positionning, up to his neighborhood. If the user does not want to be positionned, a way must found to locate him even without his knowledge.
The user must be able to modify his GPS position in his profile.
```

1. Localisation via IP  
[Voir dans le code](./web/static/js/profile.js)

```bash
# `profile.js > getSilentLocation > getIPLocation`
# liste des providers : 
#	- ipapi.co
#	- freegeoip.app
#	- ipgeolocation.io
# fallback `Paris, France` si erreur
```

### Resultat - ✅ OK

</details>

<br>

---

<br>

<details>
<summary>Profile suggestion + Research + Sort and filters</summary>

```md
The user must be able to easily get a list of suggestions when connecting that match his profile.
Suggested profiles must be consistant with sexuality. If the sexual orientation isn’t specified, the user will be considered bi-sexual.
Check with the student that profile suggestions are weighted on three criterias:
- Same geographic area as the user.
- With a maximum of common tags.
- With a maximum fame rating.
Ask the student to explain his strategy to display a list of relevant suggestions.

The user must be able to run an advanced research selecting one or a few criterias such as:
- A age gap.
- A fame rating gap.
- A location.
- One or multiple interests tags.

The suggestion list as well as the resulting list of a search must be sortable and filterable by:
- Age.
- Location.
- Fame rating.
- Tags.
```

1. Suggestions et recherche par filtre  
```bash
# a tester dans l'UI (page Explorer)
```

2. Choix de robustesse = si pas d'orientation, membres inaccessibles  
```bash
# a tester dans l'UI :
#	- desactiver Orientation Sexuelle (page Mon Profil)
#	- chercher membre (page Explorer) -> warning profil incomplet
```

### Resultat - ✅ OK

</details>

<br>

---

<br>

<details>
<summary>Profile of other users</summary>

```md
A user must be able to consult the profile of other users, that must contain all the information available about them, except for the email address and the password.
The profile must show the fame rating and if the user is connected and if not see the last connection date and time.
```

1. Visiter un profil user  
```bash
# a tester dans l'UI (page Explorer -> Voir le profil)
#	- checker toutes les infos visibles
```

### Resultat - ✅ OK

</details>

<br>

---

<br>

<details>
<summary>Connexion between users + Report et bloking + Chat + Notifications</summary>

```md
A user can like or unlike the profile of another user. When two people like each other, we will say that they are connected and can be able to chat.
A user that doesn't have a profile picture can't like another user.
The profile of other users must clearly display if they're connected with the current user or if they like the current user.
```

1. Test like/unlike/match/block/chat user  
```bash
# a tester dans l'UI (page Explorer)
#	- verifier visites/likes dans page Mon Profil
#	- verifier notifications visites/likes
#	- verifier notifications chat
#	- apres block, verifier notifications/visites/likes
```

### Resultat - ⏳ a corriger

- Signaler user : `400 Bad Request`

</details>

<br>

---

<br>

<details>
<summary>Compatibility</summary>

```md
Is the website compatible with Firefox (>= 41) and Chrome (>= 46)?
Features described above work correctly with no warnings, errors, or weird logs?
```

1. Test sur plusieurs navigateurs  
```bash
# Brave (version 1.81.137 official build)
# Chromium (version 139.0.7258.158)
# Firefox (version 142.0.1)
```

### Resultat - ✅ OK

</details>

<br>

---

<br>

<details>
<summary>Mobile</summary>

```md
Is the website usable on a mobile and on very small resolution? Is the site layout correctly displayed?
```

1. Test mobile  
```bash
# touche F12 ou clic droit > inspecter
# changer disposition vers mobile
```

### Resultat - ✅ OK

</details>

<br>

---

<br>

<details>
<summary>Security</summary>

```md
XSS / CSRF / TGIF / WYSIWYG / TMTC / TMNT...
The subject insisted on that point: the website must be secured.
Check at least the following points:
- Passwords are encrypted in the database.
- Forms and uploads have correct validations. Scripts can not be injected.
- SQL injection isn't possible. (try to login with `blabla' OR 1='1` as a password)
```

1. Voir 1ere rubrique

### Resultat - ✅ OK

</details>

<br>

---

<br>

## Bilan de l'evaluation
 
### Console logs

**A corriger**  
Erreur visible :
- signaler user : `400 Bad Request`

### Design

**A corriger**  
Suggestions de tags :  
- dropdown de tags deja existants OU suggestion selon les seeds generees au demarrage    
