# Mihari

Mihari (見張り, « sentinelle » en japonais) est une application de bureau qui fait tourner des LLM en local pour relire les merge requests GitLab et écrire des tests. Elle relit le code le jour et rédige des tests la nuit, sans que le code ne quitte la machine.

> Projet en cours de développement (V1). La supervision centralisée n'est pas encore implémentée.

## Fonctionnalités

### V1

- Configuration des profils de modèles : un modèle de journée et un modèle de nuit, avec le nombre de jobs en parallèle pour chacun
- Planning : plages horaires et profil actif sur chaque plage
- Connexion à une instance GitLab : URL, compte bot, token stocké dans le trousseau du système
- Détection des merge requests à relire par interrogation de l'API GitLab, sans webhook
- Review automatique des merge requests et publication des commentaires
- Écran d'état : modèle chargé, jobs en cours et en attente

### Prévu

- Écriture de tests unitaires et e2e la nuit, dans un conteneur basé sur l'image CI du projet
- Supervision centralisée de plusieurs machines (bail signé, configuration imposée, tableau de bord)
- Détection des usages hors périmètre
- Paquets d'installation signés pour macOS, Linux et Windows

## Architecture

```
┌──────────────────────────────┐
│ Interface (Svelte + TS)      │
└──────────────┬───────────────┘
               │ bindings Wails
┌──────────────▼───────────────┐
│ Cœur (Go)                    │
│  - configuration             │
│  - planificateur jour/nuit   │
│  - polling GitLab            │
│  - file de jobs (SQLite)     │
│  - moteurs d'inférence       │
└───────┬──────────────┬───────┘
        │              │ API compatible OpenAI
        │      ┌───────▼────────────────┐
        │      │ vllm-mlx (macOS)       │
        │      │ llama.cpp (Linux/Win)  │
        │      └────────────────────────┘
        │ API REST (sortant uniquement)
┌───────▼──────────────────────┐
│ GitLab                       │
└──────────────────────────────┘
```

Principes :

- Le cœur ne parle jamais directement à un runtime : chaque moteur d'inférence implémente la même interface (démarrer un modèle avec N slots, arrêter, vérifier l'état, exposer l'URL de l'API).
- Un seul modèle chargé par profil, avec plusieurs slots pour traiter les reviews en parallèle.
- Toutes les connexions partent de la machine : aucun port n'est ouvert.
- Le serveur d'inférence n'écoute que sur `127.0.0.1`.

## Stack

| Partie | Technologie |
|---|---|
| Application de bureau | [Wails v3](https://v3alpha.wails.io/) (bêta) |
| Interface | Svelte + TypeScript + Vite |
| Cœur | Go |
| Stockage | SQLite (`modernc.org/sqlite`, sans CGO) |
| Secrets | Trousseau du système (`zalando/go-keyring`) |
| Inférence macOS | MLX via vllm-mlx |
| Inférence Linux et Windows | llama.cpp |

## Prérequis

- Go 1.25 ou plus récent
- Node.js et npm
- CLI Wails v3 :

```
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

Selon le système :

- **Arch Linux** : `gtk4` et `webkitgtk-6.0`
- **macOS** : Xcode Command Line Tools
- **Windows** : WebView2 (inclus dans Windows 11)

Vérifier l'environnement :

```
wails3 doctor
```

## Développement

Lancer l'application en mode développement, avec rechargement à chaud :

```
wails3 dev
```

Compiler un binaire de production :

```
wails3 build
```

Le binaire est généré dans `bin/`.

### Tester sans le GitLab de production

Lancer une instance GitLab CE locale avec Docker, puis y créer un projet de test et un compte bot :

```
docker run -d --name gitlab -p 8929:80 -v gitlab-data:/var/opt/gitlab gitlab/gitlab-ce:latest
```

Prévoir environ 4 Go de RAM pour l'instance.

### Tester sans gros GPU

Utiliser un petit modèle GGUF avec llama.cpp dans un profil de développement. Le moteur vllm-mlx ne se teste que sur un Mac Apple Silicon.

## Configuration

Exemple de configuration :

```yaml
profiles:
  jour:
    backend: vllm-mlx
    model: mlx-community/Qwen3.8-Flash-Next-4bit
    parallel: 7
    context_per_slot: 65536
  nuit:
    backend: vllm-mlx
    model: Vontra/MiMo-V2.6-Flash-RL-MLX-4bit-MTP
    parallel: 3
    context_per_slot: 131072
schedule:
  - days: [mon, tue, wed, thu, fri]
    from: "08:30"
    to: "17:00"
    profile: jour
  - days: [mon, tue, wed, thu, fri, sat, sun]
    from: "17:00"
    to: "08:30"
    profile: nuit
gitlab:
  url: https://gitlab.example.com
  bot_username: review-bot
  poll_interval: 30s
```

Le token GitLab n'est jamais écrit dans ce fichier : il est stocké dans le trousseau du système.

## Fonctionnement côté GitLab

1. Créer un compte bot sur l'instance GitLab et lui donner accès aux projets à relire.
2. Générer un token d'accès pour ce compte et le saisir dans Mihari.
3. Pour demander une review, ajouter le compte bot comme reviewer de la merge request.
4. Mihari détecte la demande au passage suivant du polling, relit le diff et publie ses commentaires.

Chaque merge request n'est relue qu'une fois par commit (clé : projet, IID, SHA).

## Structure du projet

```
.
├── build/           fichiers de packaging par plateforme
├── frontend/        interface Svelte
├── main.go          point d'entrée de l'application
├── Taskfile.yml     tâches de build Wails
└── go.mod
```

## Licence

À définir.
