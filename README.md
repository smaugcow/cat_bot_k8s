# catbot k8s

все операции необходимо производить от root
небеопасно в продакшн, но для локальных эксперементов достаточно

буду использовать контейнерный движок containerd, а также cni calico

создадит registry в yandex cloud, узнем его id, получите токен для пуша image'ей и залогиньтесь в docker
```
docker login --username oauth --password <TOKEN> cr.yandex
```

соберем образ
```
docker build -t cat_bot:1.0 .
```

сделаем название image'а, необходимое для yc
```
docker tag cat_bot:1.0 cr.yandex/<ID>/cat_bot:1.0
```

запушим
```
docker push cr.yandex/<ID>/cat_bot:1.0
```

Далее будет разворачивать кластер k8s

создадим три виртуальные машины в yandex cloud
имя хостов можем задавать любое, потому что ниже мы его будет проставлять сами, выбираем необходимую сеть, если ее нет, то создаем, добавляем ssh ключи по желанию

буду показывать пример для 3 машин Linux Ubuntu из yandex cloud

устанавливаем необходимые пакеты

```
apt update && apt -y upgrade && apt -y install apt-transport-https ca-certificates curl gnupg2 software-properties-common
```

Настройка автозагрузки и запуск модуля ядра br_netfilter и overlay
```
modprobe overlay
modprobe br_netfilter
```
Разрешение маршрутизации IP-трафика
```
sysctl net.bridge.bridge-nf-call-iptables net.bridge.bridge-nf-call-ip6tables net.ipv4.ip_forward
```
Отключение файла подкачки, если делаете перезагрузку, необходимо будет выполнить команду снова
```
swapoff -a
```

проверяем что все окей 
```
lsmod | grep br_netfilter
lsmod | grep overlay
```

проверка что отключили файл подкачки
```
swapon -s
```


отключаем firewall

```
systemctl stop ufw && systemctl disable ufw
```

устанавливаем containerd

```
wget https://github.com/containerd/containerd/releases/download/v1.7.0/containerd-1.7.0-linux-amd64.tar.gz
tar Cxzvf /usr/local containerd-1.7.0-linux-amd64.tar.gz
rm containerd-1.7.0-linux-amd64.tar.gz
```

Создание конфигурации по умолчанию для containerd
```
mkdir /etc/containerd/
containerd config default > /etc/containerd/config.toml
```

Настройка cgroup драйвера
```
sed -i 's/SystemdCgroup \= false/SystemdCgroup \= true/g' /etc/containerd/config.toml
```

Установка systemd сервиса для containerd
```
wget https://raw.githubusercontent.com/containerd/containerd/main/containerd.service
mv containerd.service /etc/systemd/system/ 
```

Установка компонента runc
```
wget https://github.com/opencontainers/runc/releases/download/v1.1.4/runc.amd64
install -m 755 runc.amd64 /usr/local/sbin/runc
rm runc.amd64
```

Установка сетевых плагинов:
```
wget https://github.com/containernetworking/plugins/releases/download/v1.2.0/cni-plugins-linux-amd64-v1.2.0.tgz
mkdir -p /opt/cni/bin
tar Cxzvf /opt/cni/bin cni-plugins-linux-amd64-v1.2.0.tgz
rm cni-plugins-linux-amd64-v1.2.0.tgz
```

Запуск сервиса containerd
```
systemctl daemon-reload
systemctl enable --now containerd
```

 Проверка доступности сокета containerd

```
crictl --runtime-endpoint unix:///var/run/containerd/containerd.sock version
```

Проверка возможности запуска контейнеров с помощью containerd**

```
ctr images pull docker.io/library/hello-world:latest
ctr run docker.io/library/hello-world:latest hello-world
```

устанавливаем кубер

```
curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.29/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.29/deb/ /' | tee /etc/apt/sources.list.d/kubernetes.list
apt update && apt -y install kubelet kubeadm kubectl && apt-mark hold kubelet kubeadm kubectl
kubeadm init --pod-network-cidr=10.244.0.0/16
```


делаем мастера на мастер ноде
```
kubeadm init --pod-network-cidr=10.244.0.0/16
```

тащим на воркеры

```
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
```

джойним ноды

```
kubeadm join 172.30.0.5:6443 --token <TOKEN> --discovery-token-ca-cert-hash sha256:<HASH>
```

calico, чтобы была сеть

```
kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.27.3/manifests/calico.yaml
```

проверяем что calico задеплоился

```
kubectl get pods --all-namespaces
```

делаем reboot
```
reboot
```

проверяем статус нод на мастере

```
kubectl get node
```

Запуск пода в интерактивном режиме.

```
kubectl run -it --rm ubuntu --image=ubuntu -- apt update
# проверяем что есть сеть
apt update
# если пакеты тянутся, то окей
```

делаем секрет, чтобы можно было ходить в реджистри

```
kubectl create secret docker-registry yandex-registry --docker-server=https://cr.yandex --docker-username=oauth --docker-password=<TOKEN>
```

делаем деплоймент

```
cat <<EOF | tee /home/catbot/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catbot
spec:
  replicas: 1
  selector:
    matchLabels:
      app: catbot
  template:
    metadata:
      labels:
        app: catbot
    spec:
      imagePullSecrets:
      - name: yandex-registry
      containers:
      - name: catbot
        image: cr.yandex/<REGISTRY_ID>/cat_bot:1.0
EOF

kubectl apply -f deployment.yaml
```

проверяем что деплоймент создал под и наш бот работает
```
kubectl get all
```
