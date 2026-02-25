1.  (Build):
Bash

docker build -t forum-app .

1.  (Run):
Bash

docker container run -p 8080:8080 -v "$(pwd)/forum.db":/app/forum.db forum-app