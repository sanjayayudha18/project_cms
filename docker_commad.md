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




