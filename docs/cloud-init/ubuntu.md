# Ubuntu — cloud-init

Mirror: `https://mirror.scalewithus.com`

## amd64

```yaml
#cloud-config
apt:
  primary:
    - arches: [default]
      uri: https://mirror.scalewithus.com/ubuntu
  security:
    - arches: [default]
      uri: https://mirror.scalewithus.com/ubuntu-security
```

## arm64 / ports

```yaml
#cloud-config
apt:
  primary:
    - arches: [arm64, armhf, ppc64el, riscv64, s390x]
      uri: https://mirror.scalewithus.com/ubuntu-ports
    - arches: [default]
      uri: https://mirror.scalewithus.com/ubuntu
  security:
    - arches: [arm64, armhf, ppc64el, riscv64, s390x]
      uri: https://mirror.scalewithus.com/ubuntu-ports
    - arches: [default]
      uri: https://mirror.scalewithus.com/ubuntu-security
```

## Verify

```bash
cloud-init status --wait
apt-get update
```
