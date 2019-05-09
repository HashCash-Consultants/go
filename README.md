# Aurora setup

To build aurora you must have go and dep in your ubuntu enivironment. To install go you need to perform  below steps and go version must be >=1.9


# Install go


- sudo curl -O https://storage.googleapis.com/golang/go1.9.1.linux-amd64.tar.gz
- sudo tar -xvf go1.9.1.linux-amd64.tar.gz
- Open vi .profile file and add following lines:
```ssh
PATH="$HOME/bin:$HOME/.local/bin:$PATH"
export GOPATH=$HOME/go
export PATH=${GOPATH}/bin:${PATH}
and save the file
```
- Open vi .bashrc and add following lines
```ssh
export GOPATH=$HOME/go
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin
```
- To refresh the files execute following commands:
```ssh
source ~/.profile
source  ~/.bashrc
```
- Check go version it should be - 
```ssh
$ go version
```

# Install dep

- curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
- Check go version it should be - 
```ssh
$ dep version

```

# Install aurora

- mkdir github.com/hcnet in /home/ubuntu/go/src
- go to /home/ubuntu/go/src/github.com/hcnet and execute following command:
- git clone https://github.com/HashCash-Consultants/go.git
- go to /home/ubuntu/go/src/github.com/hcnet /go and execute following command:
```
$ sudo apt-get install mercurial
```

```ssh
$ dep ensure –v
```
- go to /home/ubuntu/go/src  and execute following command:
```ssh
$ go install -ldflags "-X github.com/hcnet/go/support/app.version=aurora-0.16.0" github.com/hcnet/go/services/aurora/
```
- after running above command you check aurora build in <Your_dir>/go/bin folder and you can check aurora version by  following command :
```ssh
$ ./aurora version

```
