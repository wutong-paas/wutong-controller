# Wutong Controller

## controllers

- service-combiner
- telepresence-rbac
- ...

## Install

资源将在 `wt-system` 命名空间中创建，请确保该命名空间已经存在。

```bash
kubectl apply -f https://raw.githubusercontent.com/wutong-paas/wutong-controller/master/deploy/manifest.yaml
```

## Uninstall

```bash
kubectl delete -f https://raw.githubusercontent.com/wutong-paas/wutong-controller/master/deploy/manifest.yaml
```