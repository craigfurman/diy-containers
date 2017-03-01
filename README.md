# diy-containers

Onboarding work for [Garden](https://github.com/cloudfoundry/garden-runc-release).
Contains code from https://www.infoq.com/articles/build-a-container-golang.

## TODO

1. faster CoW FS (aufs?)
1. memory restriction through cgroups
1. what other cgroups would be good to use?
1. ipc / network namespaces

## Notes

1. use cgroup.procs instead of tasks
