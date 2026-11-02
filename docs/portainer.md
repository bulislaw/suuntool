# Raspberry Pi / NAS deployment

The container serves Streamable HTTP MCP at `/mcp` on port 8080. The CLI's
stdio transport still works. One Suunto account is shared by all clients of
this service; every client with the bearer token can access that account's tools.
Read tools are enabled by default.

## Quick setup with Portainer

1. Push these files to your GitHub repository. The Pi builds the application
   image from source during deployment and stores it locally as `suuntool:local`.
   Docker downloads the Go/Alpine base images and Go dependencies for the build;
   no application image is published or pulled from a registry.
2. In your **Docker Standalone** environment in Portainer, select **Stacks →
   Add stack → Repository**. Name it `suuntool`, enter your GitHub repository URL,
   set the reference to `refs/heads/main` (or your deployment branch), and the
   Compose path to `compose.yaml`. Private repositories also need Git credentials.
3. Add these stack environment variables, then deploy:

   | Variable | Value |
   | --- | --- |
   | `SUUNTOOL_MCP_TOKEN` | A random secret; generate with `openssl rand -hex 32` |
   | `SUUNTOOL_BIND_ADDRESS` | Your Pi's LAN or VPN IP, e.g. `192.168.1.20` |
   | `SUUNTOOL_PORT` | `8080` (optional) |

   The default bind address is `127.0.0.1`, for a proxy on the host. To connect
   directly from other machines, set the LAN/VPN IP explicitly. An existing
   Docker reverse proxy can use a shared Docker network and `suuntool:8080`;
   adapt the Compose networks to your proxy's network.
4. Open the service container's **Console** in Portainer, connect using `/bin/sh`
   as the default container user, and run:

   ```sh
   suuntool login --email you@example.com --quiet --output /dev/null
   ```

   Enter your Suunto password at the hidden prompt. The command stores the
   session in the persistent volume; the password is not stored. Suppressing
   the login response avoids displaying the returned session key. Restart the
   service container after login: MCP reads the session at startup. Repeat login
   and restart if tools later report `AUTH_EXPIRED`.
5. Configure an MCP client that supports Streamable HTTP and bearer headers:

   - URL: `http://192.168.1.20:8080/mcp` (substitute your IP/port)
   - Header: `Authorization: Bearer <SUUNTOOL_MCP_TOKEN>`

   Use a trusted LAN/VPN, or terminate HTTPS at your existing reverse proxy.
   Pass the Authorization header through to the service. This server uses a
   pre-shared token, not an OAuth login flow. Clients requiring OAuth-only remote
   servers will need a compatible gateway. Portainer showing **healthy** means
   the HTTP process is live; it does not confirm a valid Suunto login.

Portainer clones the Git repository and reads the Compose path, as described in
the [Portainer stack documentation](https://docs.portainer.io/user/docker/stacks/add).
The Compose file uses `build.context: .` and `pull_policy: build`, so the image
is built from the checked-out repository on the Docker host. Use Portainer with
access to the Pi's local Docker socket and repository build context. Some
Portainer Agent setups cannot access Git build contexts; if deployment reports
that the Dockerfile/build path is missing, use the local socket environment or
the on-Pi Compose commands below. This setup targets Docker Standalone, not Swarm.
The first build can take several minutes on a Pi; subsequent builds reuse Docker
layers and the downloaded Go modules when `go.mod` and `go.sum` are unchanged.

## Persistent session and cache

The stack creates a named volume, normally `suuntool_data`, mounted at `/data`:

| Container path | Contents |
| --- | --- |
| `/data/config/suuntool/session.json` | Suunto session credentials |
| `/data/cache/suuntool/v1/<account hash>/` | Full downloaded SML, FIT, and guide archives |

`XDG_CONFIG_HOME` and `XDG_CACHE_HOME` point at these directories. The image runs
as UID/GID `10001:10001`; a new named volume receives the image's directory
ownership. If substituting a NAS bind mount, create the directories and give
that UID/GID write access before deployment. Keep session storage private.

The same account-scoped disk cache is used across tool calls, separate MCP
clients, and container restarts/replacements. SML stream filters/downsampling
read the cached full artifact, so changing filters does not download it again.
Lists, metadata, and other mutable responses remain live API requests. Writes
invalidate affected artifacts. Cache entries have no automatic expiry or size
limit; back up `/data` and monitor disk usage. Keep the stack name and volume
when redeploying. Removing the volume deletes both login and cached artifacts.

The default command enables `--verbose`. In Portainer logs, repeat the same
`workouts_sml` call with a small `streams` selection, or a FIT/guide download:
the first uncached read reports `server requests=1, cache hits=0`, and subsequent
reads report `server requests=0, cache hits=1`. From the container console:

```sh
suuntool cache status
suuntool cache clear       # optional: remove only this account's artifact cache
```

If you already use the CLI elsewhere, you can migrate its session and cache
into the corresponding volume directories, preserving private permissions and
ownership, then restart the service. The cache is partitioned by account identity.

## Updates and tool permissions

Push source changes to your deployment branch, then update/redeploy the Git
stack in Portainer to check out the changes and rebuild the local image. The
Compose `pull_policy: build` requests a build on deployment. Keep the same stack
name and data volume. Back up the volume before upgrades. For a reproducible
source version, deploy a Git tag or commit supported by your Portainer version.

To enable writes, add this service-level Compose setting in your deployment branch:

```yaml
command: ["mcp", "--listen", ":8080", "--verbose", "--allow-write"]
```

Add `--allow-destructive` as well if you want delete tools. Everyone holding the
MCP token receives these permissions. Rotate the token by changing the stack
variable and redeploying. MCP cannot perform interactive login.

## Build directly on the Pi (or test on a Mac)

The Dockerfile builds natively for the host architecture. Use a supported
64-bit Raspberry Pi OS installation for the Pi; on an ARM Mac the local Docker
build produces a Linux ARM64 image too. No Go installation is needed on the host.
Clone the repository on the machine running Docker, then:

```sh
cp .env.example .env
# Edit .env: set a random token and the desired bind address.
docker compose up -d --build
docker compose exec suuntool suuntool login --email you@example.com --quiet --output /dev/null
docker compose restart suuntool
```

To update a checkout on the Pi:

```sh
git pull --ff-only
docker compose up -d --build
```

The session and cache remain in the named volume while the image is rebuilt.
