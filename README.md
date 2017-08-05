# Paste: a secure personal pastebin
Written in [Go](https://golang.org/), hosted on [Google App Engine](https://cloud.google.com/appengine/).

## Features
- Requires Google authentication to create pastes (to deter spammers/ abuse)
- Automatic language detection/ syntax highlighting via highlight.js
- Solarized color theme

## Development
Local Development:
```
dev_appserver.py app.yaml
```

Deploy:
```
gcloud --project dsouza-paste app deploy
```

Update Indexes:
```
# Create new indexes
gcloud --project dsouza-paste datastore create-indexes index.yaml

# Delete old/ unused indexes
gcloud --project dsouza-paste datastore cleanup-indexes index.yaml
```

