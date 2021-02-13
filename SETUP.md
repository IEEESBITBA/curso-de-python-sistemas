# Setting up the server
So you got tasked with maintaining this project for future generations. Don't worry about getting it up and running from scratch, that'll be explained here.

This guide is designed to get a **production ready** server up and running.

## Requirements
* Have a linux machine (secure running of code and ease of setup)
* Have `git` installed.(run `sudo apt install git`)
* Have bought a domain such as `whittileaks.com`

## Overview
* We'll be cloning the repo to get the `assets` folder in the same directory as the binary.
* We'll download the binary. Alternatively you can build it but you'll need to follow steps 1-6 in [README](./README.md)
* We'll install Docker and create a container for our database (Postgres)
* We'll install [`soypat/gontainer`](https://github.com/soypat/gontainer) to run python code securely
    * We'll setup a container to securely run Python using an Alpine linux distro
* We'll configure environment variables so the server operates according to our needs
* We'll install Nginx. Nginx will serve as a reverse-proxy. It'll handle requests and serve our site according to SSL protocol (https://)
* We'll use letsencrypt to configure Nginx to serve SSL protocol

## Installing the server

### Setting up the binary
Open terminal navigate to your desired folder and clone the repo. The commands below create a `src` folder in your $HOME folder in linux and then clone the repository to folder `curso`
```console
mkdir ~/src
cd ~/src
git clone \
 https://github.com/IEEESBITBA/Curso-de-Python-Sistemas/ curso
```

Now download the binary and place it in this `curso` folder. The binary only needs you to move the `assets` folder. You can find the latest release [here](https://github.com/IEEESBITBA/Curso-de-Python-Sistemas/releases/). Make sure to download `amd64-linux` version.

The binary is ready to run though it does not a have an SQL database connection ready yet.

### Docker install and Database setup
We'll use Docker to run our Postgres database. First [install Docker](https://docs.docker.com/engine/install/ubuntu/). First open a console and change to super user. You need to be a super user to run and manage ANY Docker instance.

```console
sudo su
```

write password, now run the following. You can copy paste each block of code to console. On linux you use <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>V</kbd> to paste to console!

```bash
#update apt
sudo apt-get update
```

```bash
# install barebones stuff if you dont have them
sudo apt-get install \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg-agent \
    software-properties-common
```

```bash
# add docker GPG key
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
```
```bash
# add Docker repository
sudo add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"
```

```bash
# update apt again
sudo apt-get update
```

```bash
# install Docker
sudo apt-get install docker-ce docker-ce-cli containerd.io
```

Now running `docker` should output lots of options.

Next step is to get the Postgres instance up and running. Running the following command for the first time will have the following effects:

* Docker will download the Postgres image if not already on your machine
* Docker will instantiate a container with the postgres image named `forum`
* Docker wll create a database for you with the following parameters:
    * Password will be `1337`
    * DB username will be `pato`
    * DB name will be `curso`
    * Postgres will be accessible on port `5432`


The binary you downloaded earlier probably has these parameters set, hard coded! If you wish to use different parameters you must:
* Follow steps 1-6 in [README](./README.md)
* Modify [`database.yml`](./database.yml) to your desired needs (step 6)
* Build the application by running `GOOS=linux GOARCH=amd64    buffalo build  -o bin/curso-linux-amd64 .`
```bash
# Read above before changing parameters!
docker run --name forum  \
-e POSTGRES_PASSWORD=1337 \
-e POSTGRES_USER=pato \
-e POSTGRES_DB=curso \
-p 5432:5432 -d postgres
```

Now we create our tables by migrating. Our binary has a subcommand allocated for just this purpose. Go to the `curso` folder we created earlier and run our binary with `migrate` argument. You need to be a super user to run migrations using the binary due to a bug in the indexer (probably `./actions/search.go:line 29`). *This is probably for the best though as the server **needs** to be run as super user in production due to it running containers for remote Python code execution using chroots.*

```bash
sudo ./curso-linux-amd64 migrate
```

Great! Now you can run the server locally! Start the server (as a super user) and go to [http://127.0.0.1:3000](http://127.0.0.1:3000)
```bash
sudo ./curso-linux-amd64
```
| Keep in mind you'll have to run the following command to run the database every time you boot your PC back up|
|------|
|```docker restart forum```|

(`forum` is name of the container)

### Configuring environment variables
Look at all those envars. This is where you...
1. Most importantly, setup OAuth keys for google, github and facebook
2. Setup [Gontainer](https://github.com/soypat/gontainer)'s default filesystem (where it chroots to)
3. Configure server parameters, internet stuff, how long it runs python code before killing the process, SMTP mail server for notifying users of comments and successful attempts at Python code challenges.

If at any time you feel curious as to what an envar does, just open up VScode or similar editor, <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>F</kbd> and search it's name in all files. They are usually used only once and they don't persist, that is, they are not saved in a variable to be used several lines later.

```shell script
# Required for OAuth2 (Default uses google as provider)
GGL_SECRET_FORUM=xxxxxxxxx # This is google's secret API token (client secret)
GGL_KEY_FORUM=1113333333-xXxXxXXX  # This is google's client ID
# See other provider env in actions/auth.go under init() function

# Optional
# Nothing below this line is required to make the server work
# --------------
# Warning! gontainer requires root privilges just like any other application which performs a chroot
# It is suggested test development depend on the system's python (set GONTAINER_FS="" or don't set it)
GONTAINER_FS=/mnt/cursoFS # Path to mounted-point of linux filesystem with python3 installation
FORUM_HOST=https://my.site.com  # If hosting on non-local address this is required for proper callback function
PORT=3000 #Default
ADDR=127.0.0.1 # Default
FORUM_LOGLVL=info # Default
PY_TIMEOUT=500ms # duration format. decides max cpu time for python interpreter (default 500ms)
# SMTP server (as would be set in ~/.bashrc)
# Set this up if you want replies to trigger notification Email
CURSO_SEND_MAIL=true
SMTP_PORT=587 #for google
SMTP_HOST=smtp.gmail.com
SMTP_USER=miusuario@itba.edu.ar
SMTP_PASSWORD=abc123
CURSO_MAIL_NOTIFY_REPLY_TO=donotreply@ieeeitba.org
CURSO_MAIL_NOTIFY_MESSAGE_ID=ieeeitba.org
CURSO_MAIL_NOTIFY_IN_REPLY_TO="Curso de Python 2020 - 2C"
CURSO_MAIL_NOTIFY_LIST_ID="Notificaciones Foro <cursos.ieeeitba.org>"
CURSO_MAIL_NOTIFY_LIST_ARCHIVE="https://curso.whittileaks.com"
CURSO_MAIL_NOTIFY_SUBJECT_HDR="Te han respondido - Curso de Python"
CURSO_MAIL_NOTIFY_FROM=cursos_IEEE@itba.edu.ar # Configurar alias para usar este campo. Si no configuro el alias: usar ${SMTP_USER} para esta variable
```

If you wish to automatically set them everytime you start up your linux machine, head over to your `/etc/environment` file (see [stackoverflow](https://stackoverflow.com/questions/14637979/how-to-permanently-set-path-on-linux-unix)) and copy paste above codeblock at the end. You can do this opening the file as super user using an editor like `nano` (remember, <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>V</kbd> pastes text into console)
```bash
sudo nano /etc/environment
```
**how to save file on nano**: 
1. Once done editing <kbd>Ctrl</kbd>+<kbd>X</kbd> begins exit
2. <kbd>Y</kbd> tells nano you want to save your changes
3. <kbd>Enter</kbd> tells nano you want to save with same name. It then exits. Your progress is saved!

## [Gontainer](https://github.com/soypat/gontainer) installation and setup

As mentioned earlier, [Gontainer](https://github.com/soypat/gontainer) runs Python code safely on the machine. It is **not** strictly required to run the server, but would you trust a stranger with the keys to your house, car and gunsafe?

FAQ: 
* Q:Why Gontainer over Docker?
    * A: Python's workspace is not obfuscated behind needing to run a Docker command, allowing one to easily change files accessible to the container's users. Also gontainer has minimal overhead, allowing for faster Python runs which would otherwise contribute to the time it takes for the process to timeout.

### Download Alpine Linux (or your favorite linux distro filesystem)
Get it here https://alpinelinux.org/downloads/. I strongly recommend the **Mini root filesystem** (2.6MB). If you run a 64bit computer (99.99% likely) then x86_64 is for you.

### Gontainer Installation
[Download latest gontainer release](https://github.com/soypat/gontainer/releases/tag/v2.0.0) or build source code, you know the drill. Gontainer is **linux only**. Windows has no such thing as `chroot`s.

Move the binary (executable) to some directory on your `$PATH`. If you are not sure where, just move it to `/bin`:

```bash
# copy gontainer to /bin directory
sudo cp gontainer /bin/gontainer
```

after moving it, allow it to be executed
```bash
sudo chmod +x /bin/gontainer
```

Run the following to check if gontainer has been succesfully installed. You should see a list of helpful notes.
```bash
gontainer --help
```
### Creating a linux filesystem (the "jail")
We will now create the virtual "disk" where Python will run. We will give it some characteristics:
* Loopback file: It will have a limited size. So as to prevent our main computer's memory from being attacked
* Python installation: It should run python3 code!

We start by creating a file called `mnt-vfs.sh` which will contain the script for creating a filesystem. We will create it in our `~/src/` folder. (or you can download it from https://github.com/soypat/gontainer/blob/master/mnt-vfs.sh)

`touch ~/src/mnt-vfs.sh`

Add the following as content to the file

```bash
#!/bin/bash
# Patricio Whittingslow. Courtesy of http://souptonuts.sourceforge.net/quota_tutorial.html
TYPE="ext3" #Type can be ext2, ntfs, ext4 you name it
MNTDIR="/mnt/$1"
NAME="$1.${TYPE}"

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
mkdir -p $MNTDIR
BLOCKCOUNT=`expr $2 \* 1000 \* 1000 / 512`
echo "vfs of size ${2}MB"
# We first make the image of the quota size
dd if=/dev/zero of=./$NAME count=${BLOCKCOUNT}
# NExt we make the filesystem on the image
/sbin/mkfs -t $TYPE -q ./$NAME -F
# append a line to fstab
#This will make the filesystem always available on reboot, plus it's easier to mount and unmout when testing. 
echo "$DIR/$NAME  $MNTDIR   $TYPE rw,loop,usrquota,grpquota  0 0" &>> /etc/fstab

mount $MNTDIR
```

We now make this file executable

```bash
sudo chmod +x ~/src/mnt-vfs.sh
```

It's usage is simple: Two arguments follow the bash call
1. Name of the filesystem (no effect on how it works)
2. Size limit in MB (Mega**bytes**)

```bash
# This creates a 2GB virtual filesystem at ~/src/cursoFS.ext4 and mounts it at /mnt/cursoFS
sudo ~/src/mnt-vfs.sh cursoFS 2000
```

Now you are ready to "install" linux on it! Extract the Alpine Linux filesystem you downloaded earlier to some **empty folder** and copy them to the mount-point. I've extracted Alpine linux to a folder called `alpine_fs` under my `~/src/` directory.

```bash
# if you named your VFS differently you'll have to change it here. You can check it's directory with command "ls /mnt" (no quotes)
sudo cp -r ~/src/alpine_fs/. /mnt/cursoFS
```

To check if you did everything correctly until now, run Gontainer on your virtual system!

```bash
sudo gontainer --chrt /mnt/cursoFS run echo "I'm working just fine!"
```
If `"I'm working just fine!"` is printed to your console, then you've succesfully deployed a container!

### Installing Python on the virtual filesystem (VFS)
Next part is conceptually tricky. We will open a console on the **inner side** of the container and install things this way.

We'll be using a minimal working console called **`ash`** that comes with all Alpine Linux distros.

We use gontainer to access this console
```bash
sudo gontainer --chrt /mnt/cursoFS run ash
```

| **DO NOT CLOSE THIS CONSOLE WINDOW UNTIL DONE CONFIGURING PYTHON**. If you close it you'll have to remember where you left off! |
|---|

You'll be presented with a colorless window with a `#` cursor in the `/usr` directory. Let's start configuring our container! 

Let's allow internet access to our container so it can download packages:
```ash
echo nameserver 8.8.8.8 > /etc/resolv.conf
```

We now install barebones python

```ash
env PYTHONUNBUFFERED=1
apk add --update --no-cache python3 && ln -sf python3 /usr/bin/python
```

Install pip, the friendly package manager
```ash
python3 -m ensurepip && pip3 install --no-cache --upgrade pip setuptools
```

Well you are pretty much set unless you want numpy and other compiled packages, in which case you'll have to install the GNU compiler collection (700MB or so)
```ash
apk add --update alpine-sdk && apk add python3-dev && apk add --update gfortran py-pip build-base
```
There's no "nice" way to install Python packages without breaking things. Python is, after all, beyond good and evil, evoking an almost *lovecraftian* vibe. If you wish to install pandas and numpy, it is recommended you install with the following flags (from some stack overflow comment)

numpy:
```bash
ARCHFLAGS=-Wno-error=unused-command-line-argument-hard-error-in-future pip install --upgrade numpy
```
pandas
```bash
ARCHFLAGS=-Wno-error=unused-command-line-argument-hard-error-in-future pip install --upgrade pandas
```
Please use this way of installing packages to avoid unwanted problems. **NOTE:** *I was unable to install numpy the original way I installed it in the year 2020. I suggest the reader search "install numpy on alpine linux" on google if they are unable to install it.*

You are ready! Exit **`ash`** by typing `exit` and <kbd>Enter</kbd>.

Test Python installation by running:
```bash
sudo gontainer --chrt /mnt/cursoFS/ run echo "print('hello world, I am in a container')" | python3
```
Should print out `hello world, I am in a container`.

Now gontainer is configured and works, set the environment variable `GONAINER_FS` in your `/etc/environment` file to the mount point! in this case, it would look like:
```bash
GONTAINER_FS=/mnt/cursoFS
```
This will enable the containerization of all Python code on the forum. You now have an *almost* production grade secure online Python interpreter.



---

## Serving SSL

Alright, now comes the tasty part. Installing and configuring Nginx to serve end-to-end encrypted webpages (SSL protocol). This prevents ominous error messages being displayed on user browsers.

### Installing Nginx
Nginx is a multi-use web-router program. We will use it's powerful reverse-proxy and SSL features.

```bash
sudo apt install nginx
```
Done

### Configure Nginx
Create a file with your website name ending in `.conf` using nano at `/etc/nginx/conf.d/`
```
sudo nano /etc/nginx/conf.d/mysite.ieee.com.conf
```

| Remember how to use Nano? There's a small explanation at the end of `setting up environment variables` section. |
|---|

Copy the following contents to the file and replace the `server_name` line with your own domain

```conf
upstream buffalo_app {
    server 127.0.0.1:3000;
}

server {
    listen 80;
    # REPLACE THE FOLLOWING LINE WITH YOUR DOMAIN!
    server_name mysite.ieee.com;

    # Hide NGINX version (security best practice)
    server_tokens off;

    location / {
        proxy_redirect   off;
        proxy_set_header Host              $http_host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_pass       http://buffalo_app;
        # wss socket if u wanna                                                              
        proxy_http_version 1.1;                                                             proxy_set_header Upgrade $http_upgrade;
      	proxy_set_header Connection "upgrade";
    }
}
```
Two tutorials to setup **letsencrypt**
* Follow the instructions on how to setup `let's encrypt` at https://certbot.eff.org/lets-encrypt/ubuntubionic-nginx

* Here's [another link to a different](https://www.digitalocean.com/community/tutorials/how-to-secure-nginx-with-let-s-encrypt-on-ubuntu-18-04) tutorial. I personally prefer the first one

You will need to have setup the domain to point to your PUBLIC IP! 

```bash
# this is a list of commands you'll probably run when creating your SSL certificate using let's encrypt. Not guaranteed to work
sudo snap install core; sudo snap refresh core
sudo apt-get remove certbot
sudo snap install --classic certbot
sudo ln -s /snap/bin/certbot /usr/bin/certbot
sudo certbot --nginx
# Test automatic renewal
sudo certbot renew --dry-run 
```

## How to setup domain (google domains)
I'll walk you through personal experience of what seems to work best for hosting a website yourself using Google Domains.

Log into [google domains](https://domains.google.com) using incognito (for some reason the site is super broken if you have multiple accounts logged in simultaneously).

1. Navigate to the DNS tab.
2. **Name servers** > set to use *Use the Google Domains name servers*
3. Scroll to **Custom resource records**
    * Add a *name*: i.e. `curso` (this will mean you will access your website from curso.yourdomain.org)
    * Set it to `A` record type (IPv4 compatibility)
    * *TTL*: `1H` works fine (1 hour)
    * Set *Data* to your public IPv4 address. You can find it running a command:

    ```bash
    curl ifconfig.me && echo ""
    ```
4. You will be able to access your server through http://curso.yourdomain.org after 2 days max although sometimes it takes just 20 minutes. remember to have your server running and Nginx `.conf` file modified.

## Thanks for reading
Hopefully you got through this without having to google too much. You can submit an issue if you find anything worth adding. 😄️