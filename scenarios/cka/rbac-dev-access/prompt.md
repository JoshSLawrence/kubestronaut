The team needs a service account that can manage deployments in the `dev`
namespace and look at the cluster's nodes, and nothing more.

Create these objects:

- ServiceAccount `deployer` in namespace `dev`
- Role `deployer` in `dev`: create, get, list, watch, update, patch and delete
  on `deployments`, and get and list on `pods`
- RoleBinding `deployer` in `dev` that binds the Role to the ServiceAccount
- ClusterRole `node-reader` that allows get and list on `nodes`
- ClusterRoleBinding `deployer-node-reader` that binds it to the ServiceAccount

Do not grant `deployer` anything beyond this. In particular it must not be able
to read secrets, work outside `dev`, or use the `cluster-admin` role.
