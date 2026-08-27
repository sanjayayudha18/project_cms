# 1st time create img

cd frontend
docker compose up -d --build

docker compose build --no-cache 
&& 
docker compose up -d

# docker compose

- Stop the old combined stack (from project root)
docker compose down

- Run backend independently
cd backend
docker compose up -d --build

- Run frontend independently
cd frontend
docker compose up -d --build

# update docker img

If you're using the root 
docker-compose.yml
 (manages both portals):

## docker compose up -d --build vendorportal-vite
## docker compose up -d --build userportal-vite

Run this from the frontend/ directory.

# Clean Build
Here's the sequence to update your running containers after code changes:

Backend:

cd backend
docker compose down
docker compose up -d --build

`````````
cd backend-cit 
docker compose down
docker compose up -d --build

Frontend:

cd frontend
docker compose down
docker compose up -d --build

Important: Start backend first since the frontend's cms-net network is set to external: true and references the backend's network (cms-backend_cms-net).

What each step does:

down — stops and removes the old containers (images stay)
up -d --build — rebuilds images from your updated code, then starts fresh containers in detached mode


If you only changed code (not Dockerfile or deps), --build is enough — no need for --no-cache. Docker layer caching will skip unchanged steps (base image, deps install) and only re-run from the COPY . . step onward. Much faster than --no-cache


