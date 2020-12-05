


docker build -t of-watchdog:build  . --no-cache
docker tag of-watchdog:build rounak316/of-watchdog:build
docker push  rounak316/of-watchdog:build 




docker build -t of-watchdog:0.8.1  - < Dockerfile.packager --no-cache
docker tag of-watchdog:0.8.1 rounak316/of-watchdog:0.8.1
docker push  rounak316/of-watchdog:0.8.1 