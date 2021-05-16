# Example usage

Create a config file (~/rr_config.yml) like this

```
user:               username
ip:                 remote_host_ip
rmt_tmp_dir:        /tmp/RRender
debug:              false
```

Then execute command as

```
remote_render -r success_list.output -e failed_list.output -c ~/rr_config.yml -d $(pwd) -s 5
```

This command will speed up videos in current dir to 5X.

To change bitrate, use `-b` instead of `-s`.
