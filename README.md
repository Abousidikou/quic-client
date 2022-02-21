# quic-client
quic-client is a utility for testing on a quic server whose code is in the .bak folder

# Go Install
  Login to your Ubuntu system using ssh and upgrade to apply latest security updates there.
  ```bash
  sudo apt-get update  
  sudo apt-get -y upgrade  
  ```

  Now download the Go language binary archive file using following link. To find and download latest version available or 32 bit version go to official [download page](
      https://golang.org/dl/
  )
  ```bash
  wget https://dl.google.com/go/go1.16.4.linux-amd64.tar.gz   
  ```

  Now extract the downloaded archive and install it to the desired location on the system. For this tutorial, I am installing it under /usr/local directory.
  ```bash
  sudo tar -xvf go1.16.4.linux-amd64.tar.gz  
  sudo rm -r /usr/local/go
  sudo mv go /usr/local 
  ```
  
Now you need to setup Go language environment variables for your project. Commonly you need to set 3 environment variables as GOROOT, GOPATH and PATH.
Create go directory
```bash
mkdir ~/go
```
Edit ~/.bashrc file
```bash 
nano ~/.bashrc
```
Add this below at the end of file
```bash
export GOROOT=/usr/local/go 
export GOPATH=$HOME/go 
export PATH=$GOPATH/bin:$GOROOT/bin:$PATH 
```
Refresh profile file to 
```bash
source ~/.bashrc
```

    
Verify the installation 
```bash
go version
```
Result
```bash
go version go1.16.4 linux/amd64
```
    
# quic-client install

Clone the repository and install
```bash
git clone https://github.com/Abousidikou/quic-client.git && cd quic-client
go install
```

# Verify Installation
```bash
quic-client -h
```
The result :
```bash
Usage of quic-client:
  -d int
    	The  number of bidirectional stream  (default 262144)
  -n int
    	The  number of bidirectional stream  (default 30)
  -p int
    	The  port to use for getting test done  (default 4447)
  -u string
    	The address to use for getting test done  (default "emes.bj")
```
