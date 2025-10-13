# logstream

Small example Go project for ingesting and storing logs.

Quick start

1. Build locally:

   go build ./...

2. Run the main program:

   go run ./cmd/logstream

3. Run tests (if any):

   go test ./...

How to upload to GitHub

Option A — Using GitHub website:

1. Create a new repository on https://github.com/new (give it a name, e.g., `logstream`).
2. On your machine, initialize git, add files, and push:

   git init -b main
   git add .
   git commit -m "Initial commit"
   git remote add origin https://github.com/<your-username>/logstream.git
   git push -u origin main

Option B — Using GitHub CLI (`gh`):

1. Create a repo from the folder:

   gh repo create <your-username>/logstream --public --source=. --remote=origin --push

2. Or create the repo on GitHub and then push the existing repo with:

   git remote add origin https://github.com/<your-username>/logstream.git
   git push -u origin main

Notes

- Make sure you don't commit secrets (API keys, credentials). Add them to `.gitignore` if needed.
- Consider adding a `LICENSE` if you want others to reuse the code (MIT is a common choice).
