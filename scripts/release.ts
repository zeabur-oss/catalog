// One stage of a release, as the shared release workflow of zeabur-oss/.github runs it, in the dev
// shell of the commit being released: `bun scripts/release.ts <stage> <version> ...`.
// The workflow itself checks the CHANGELOG.md section, signs SHA256SUMS, attests the assets and
// writes the notes. This script keeps what is Zeabur Catalog's own.
//
//   check   <version>                           the version is flake.nix's
//   build   <version> <os> <arch> <dist>        validate, render, nix build .#release into <dist>
//   publish <version> <artifacts> <published>   nothing to push: subjects.json is []
//   assets  <version> <artifacts> <assets>      copy the release's three files into <assets>
//   verify  <version> <os> <arch> <downloaded>  the checksums hold and nix build .#release is the same
//
// The release is data, the same on every platform, and one platform builds it. <artifacts> holds
// candidate-<os>-<arch>/ from build.
import { Failure } from "@zeabur-oss/release";
import { $ } from "bun";
import { cp, mkdir, readdir } from "node:fs/promises";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const files = ["bundle.tar.gz", "index.json", "checksums.txt"];

function fail(message: string): never {
  console.error(message);
  process.exit(1);
}

async function check(version: string) {
  const flake = (await $`nix eval --raw ${root}#catalog.version`.text()).trim();
  if (version !== `v${flake}`)
    fail(`the tag ${version} is not v${flake}, the version in flake.nix`);
}

async function build(dist: string) {
  await $`go run ./tools/cmd/catalog validate`.cwd(root);
  await $`go run ./tools/cmd/catalog render`.cwd(root);
  await $`nix build .#release`.cwd(root);
  await mkdir(dist, { recursive: true });
  for (const file of files) {
    await cp(join(root, "result", file), join(dist, file));
    await $`chmod u+w ${join(dist, file)}`;
  }
  // Smoke: the checksums hold, the index is JSON and the bundle carries it.
  await $`sha256sum -c checksums.txt`.cwd(dist);
  await $`jq -e .indexVersion ${join(dist, "index.json")} > /dev/null`;
  const carried =
    await $`tar -xzOf ${join(dist, "bundle.tar.gz")} index.json`.arrayBuffer();
  if (
    !Buffer.from(carried).equals(
      Buffer.from(await Bun.file(join(dist, "index.json")).arrayBuffer()),
    )
  )
    fail("the bundle's index.json is not the release's index.json");
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
  for (const file of files)
    await cp(join(artifacts, candidates[0], file), join(dir, file));
}

async function verify(
  version: string,
  os: string,
  arch: string,
  downloaded: string,
) {
  console.log(`verifying ${version} on ${os}/${arch}`);
  await $`sha256sum -c checksums.txt`.cwd(downloaded);
  // The release is what nix build .#release makes at the tag, byte for byte.
  await $`nix build .#release`.cwd(root);
  for (const file of files) {
    const built = Buffer.from(
      await Bun.file(join(root, "result", file)).arrayBuffer(),
    );
    const released = Buffer.from(
      await Bun.file(join(downloaded, file)).arrayBuffer(),
    );
    if (!built.equals(released))
      fail(`${file} differs from nix build .#release`);
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
  else if (stage === "build" && args.length === 3) await build(args[2]);
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
