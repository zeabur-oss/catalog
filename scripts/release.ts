// One stage of a release, as the shared release workflow of zeabur-oss/.github runs it, in the dev
// shell of the commit being released: `bun scripts/release.ts <stage> <version> ...`.
// The workflow itself checks the CHANGELOG.md section, signs SHA256SUMS, attests the assets and
// writes the notes. This script keeps what is Zeabur Catalog's own.
//
//   check   <version>                           the version is flake.nix's
//   build   <version> <os> <arch> <dist>        validate, render, then the bundle and the API into <dist>
//   publish <version> <artifacts> <published>   nothing to push: subjects.json is []
//   assets  <version> <artifacts> <assets>      copy the release's files into <assets>
//   verify  <version> <os> <arch> <downloaded>  the checksums hold and a rebuild gives the same files
//
// bundle.tar.gz is what nix build .#release makes. api-v<N>.tar.gz is the static API of major N,
// which `catalog api` writes; it renders charts that helm fetches, so it is built here, outside
// the nix sandbox. The pages workflow publishes it to https://oss-catalog.zeabur.com/v<N>/.
//
// The release is data, the same on every platform, and one platform builds it. <artifacts> holds
// candidate-<os>-<arch>/ from build.
import { Failure } from "@zeabur-oss/release";
import { $ } from "bun";
import { cp, mkdir, mkdtemp, readdir } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");

function fail(message: string): never {
  console.error(message);
  process.exit(1);
}

async function check(version: string) {
  const flake = (await $`nix eval --raw ${root}#catalog.version`.text()).trim();
  if (version !== `v${flake}`)
    fail(`the tag ${version} is not v${flake}, the version in flake.nix`);
}

// buildAPI writes api-v<N>.tar.gz of the static API into dir, reproducibly, and returns its name.
async function buildAPI(version: string, dir: string): Promise<string> {
  const out = await mkdtemp(join(tmpdir(), "catalog-api-"));
  await $`go run ./tools/cmd/catalog api -version ${version.slice(1)} -out ${out}`.cwd(
    root,
  );
  const majors = await readdir(out);
  if (majors.length !== 1 || !/^v\d+$/.test(majors[0]))
    fail(`catalog api wrote ${majors.join(", ")}, not one v<N> directory`);
  const name = `api-${majors[0]}.tar.gz`;
  await $`tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner --mode=u+w,go-w,a+rX -c ${majors[0]} | gzip -9n > ${join(dir, name)}`.cwd(
    out,
  );
  return name;
}

async function build(version: string, dist: string) {
  await $`go run ./tools/cmd/catalog validate`.cwd(root);
  await $`go run ./tools/cmd/catalog render`.cwd(root);
  await $`nix build .#release`.cwd(root);
  await mkdir(dist, { recursive: true });
  await cp(join(root, "result", "bundle.tar.gz"), join(dist, "bundle.tar.gz"));
  await $`chmod u+w ${join(dist, "bundle.tar.gz")}`;
  const api = await buildAPI(version, dist);
  await $`sha256sum bundle.tar.gz ${api} > checksums.txt`.cwd(dist);
}

// The catalog is files on the release; it pushes no image or chart.
async function publish(out: string) {
  await mkdir(out, { recursive: true });
  await Bun.write(join(out, "subjects.json"), "[]\n");
}

async function assets(artifacts: string, dir: string) {
  const candidates = (await readdir(artifacts)).filter((f) =>
    f.startsWith("candidate-"),
  );
  if (candidates.length !== 1)
    fail(`expected one candidate in ${artifacts}, found ${candidates.length}`);
  await mkdir(dir, { recursive: true });
  await cp(join(artifacts, candidates[0]), dir, { recursive: true });
}

async function verify(
  version: string,
  os: string,
  arch: string,
  downloaded: string,
) {
  console.log(`verifying ${version} on ${os}/${arch}`);
  await $`sha256sum -c checksums.txt`.cwd(downloaded);
  // The release is what a rebuild at the tag makes, byte for byte.
  await $`nix build .#release`.cwd(root);
  const rebuilt = await mkdtemp(join(tmpdir(), "catalog-verify-"));
  await cp(
    join(root, "result", "bundle.tar.gz"),
    join(rebuilt, "bundle.tar.gz"),
  );
  const api = await buildAPI(version, rebuilt);
  for (const file of ["bundle.tar.gz", api]) {
    const built = Buffer.from(
      await Bun.file(join(rebuilt, file)).arrayBuffer(),
    );
    const released = Buffer.from(
      await Bun.file(join(downloaded, file)).arrayBuffer(),
    );
    if (!built.equals(released))
      fail(`${file} differs from a rebuild at ${version}`);
  }
  console.log(`release ${version} verified`);
}

const [stage, version, ...rest] = process.argv.slice(2);
const usage =
  "usage: bun scripts/release.ts check|build|publish|assets|verify <version> ... (see the script's header)";
// SemVer with an optional pre-release; build metadata (+) cannot be in an image or chart tag.
if (!version || !/^v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/.test(version))
  fail(usage);
const args = rest.map((a, i) =>
  (stage === "build" || stage === "verify") && i < 2 ? a : resolve(a),
);
try {
  if (stage === "check" && args.length === 0) await check(version);
  else if (stage === "build" && args.length === 3)
    await build(version, args[2]);
  else if (stage === "publish" && args.length === 2) await publish(args[1]);
  else if (stage === "assets" && args.length === 2)
    await assets(args[0], args[1]);
  else if (stage === "verify" && args.length === 3)
    await verify(version, args[0], args[1], args[2]);
  else fail(usage);
} catch (e) {
  if (e instanceof Failure) fail(e.message);
  // The command has printed its own error.
  if (e instanceof $.ShellError) process.exit(e.exitCode);
  throw e;
}
