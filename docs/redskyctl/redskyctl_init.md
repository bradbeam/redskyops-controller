## redskyctl init

Install to a cluster

### Synopsis

Install Red Sky Ops to a cluster

```
redskyctl init [flags]
```

### Options

```
      --bootstrap-role       Create the bootstrap role. (default true)
      --extra-permissions    Generate permissions required for features like namespace creation.
  -h, --help                 help for init
      --ns-selector string   Create namespaced role bindings to matching namespaces.
      --wait                 Wait for resources to be established before returning.
```

### Options inherited from parent commands

```
      --context string        The name of the redskyconfig context to use. NOT THE KUBE CONTEXT.
      --kubeconfig string     Path to the kubeconfig file to use for CLI requests.
  -n, --namespace string      If present, the namespace scope for this CLI request.
      --redskyconfig string   Path to the redskyconfig file to use.
```

### SEE ALSO

* [redskyctl](redskyctl.md)	 - Kubernetes Exploration

