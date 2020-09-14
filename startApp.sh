export GO_ENV=production
export HOST="https://curso.whittileaks.com"
export ADDR=127.0.0.1
export PORT=3000
#sudo docker restart forum # if not started already

buffalo dev &>> ./assets/logs/curso.log & # the last ampersand pushes job to background
