# Backer

Simple utility for keeping those various config files and settings that you have scattered across your servers, backed up to remote storage.

I created this tool after a server crash left me scrambling to rebuild nginx sites and dnsmasq configurations that I'd setup years ago and forgot about.

This is *NOT* intended to be used as a full backup solution, there are much better tools for that.
This is designed to be a lightweight utility for watching a handfull of files and pushing them to a remote location whenever they change.

## Usage

### Install

#### Go package

```bash
get get -u github.com/nickrobison/backer
```

#### Debian repository

```bash
sudo apt-get install apt-transport-https # Bintray only supports https connections
echo "deb https://dl.bintray.com/nickrobison/debian {distribution} {components}" | sudo tee -a /etc/apt/sources.list
apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys 379CE192D401AB61 # We need to import the Bintray public key
sudo apt-get update && apt-get install backer
```

### Configuring

Backer has a few configuration options that need to be set.
If you installed the Debian package, the default config is located in `/etc/backer/config.json`.
Otherwise, create a `config.json` file and point Backer to it.

```json
{
    "deleteOnRemove": true, // When a file is removed from the system, delete its remote copy (Not implemented yet)
    "deleteOnShutdown": false, // Delete the remote files when a shutdown occurs (Not implemented yet)
    "watchers": [
        {
            "bucketPath": "",
            "path": ""
        }
    ], // Array of files paths to watch, along with a root directory to store files in
    "s3": {
        "versioning": true, // Enable versioning in the S3 bucket
        "reducedRedundancy": true, // Use reduced redundency storage
        "region": "us-west-2", // AWS region
        "bucket": "", // Name of bucket to use
        "bucketRoot": "", // Directory within the bucket to store the files
        "credentials": {
            "AccessKeyID": "",
            "SecretAccessKey": "",
        } // AWS credentials
    }
}
```

### Running

This tool has two parts, a backend daemon and a frontend CLI.

The daemon needs to be running in order for the CLI to have something to communicate with.

#### Start the Daemon

```bash
backer --daemon --config={path to config file}
```

If you're using the Debian package, you can use systemd to start everything.

```bash
systemctl start backer
```

### Using the CLI

Most of the CLI is unimplemented right now, but you can at least get a list of watcher roots, so that's nice.

```bash
backer list watchers
```

## TODO list

This is a really early stage release, lots of things still left to do.

Right now, you can only upload files, you can't delete them, and you can't download them directly with Backer. You'll need to rely on other tools for that.

- [ ] File downloading
- [ ] File deletion
- [ ] Multiple backends
    - [ ] SCP
    - [ ] FTP
- [ ] Full CLI support
- [ ] Windows support