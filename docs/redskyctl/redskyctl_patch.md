## redskyctl patch

Create a patched manifest using trial parameters

### Synopsis

Create a patched manifest using the parameters from the specified trial

```
redskyctl patch [flags]
```

### Options

```
      --file strings      experiment and related manifests to patch, - for stdin
  -h, --help              help for patch
      --trialnumber int   trial number (default -1)
```

### Options inherited from parent commands

```
      --context name        the name of the redskyconfig context to use, NOT THE KUBE CONTEXT
      --kubeconfig file     path to the kubeconfig file to use for CLI requests
  -n, --namespace string    if present, the namespace scope for this CLI request
      --redskyconfig file   path to the redskyconfig file to use
```

### SEE ALSO

* [redskyctl](redskyctl.md)	 - Kubernetes Exploration

