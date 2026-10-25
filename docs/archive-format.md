# Archive format and compatibility

[Back to the README](../README.md)

A `.diz` file is a ZIP archive. The extension is a convention; ZIP tools can inspect and extract its entries.

## Layout

```text
images.diz
├── .diz/
│   ├── manifest.json
│   ├── repositories
│   └── ... image configuration and layer files ...
└── .hashes
```

Docker image-export entries are stored under `.diz/`. `manifest.json` connects each image's tags, config file, and layers. `repositories` contains legacy repository metadata. `.hashes` maps ZIP entry names to SHA-256 hashes of their uncompressed contents.

Images may reference legacy paths such as `<config>.json` and `<layer>/layer.tar`, or current Docker export paths such as `blobs/sha256/<digest>`. Archive filtering preserves the referenced files in either layout.

Files outside `.diz/` can also be carried through archive-to-archive copies and exposed through `/get/`. Treat the complete archive as the content being shared when running the server.

## Creating, copying, and restoring

1. **Create:** Docker exports the selected images as a tar stream. diz compresses the entries into ZIP and records their hashes.
2. **Copy or update:** existing ZIP entries can be copied as compressed bytes. Selected image tags are merged into freshly written manifest files.
3. **Restore:** diz selects the referenced files, recreates a Docker-compatible tar stream, and sends it to Docker's image-load API.

Shared entry names are written once. This is filename-based reuse of Docker's image data, not a general-purpose content-addressed deduplication system. Updates produce a new archive and need disk space for that output.

The `.hashes` file is an index used for blob lookup. It is not a signature or proof that an archive came from a trusted publisher.

## Serving registry manifests

The server synthesizes Docker schema 2 manifests from the stored image records and file hashes. Their digests identify those generated manifests, so they need not match the digests used by the upstream registry. `-digest` tag-file rewrites are intended for use with a diz-served archive.

The embedded server supports image pulls and archive inspection. It has no push API, authentication, TLS, or configurable listen address. See [the server documentation](cli.md#serve).

## Parallel compression

[pgzip](https://github.com/klauspost/pgzip) divides large inputs into blocks and compresses them concurrently. Its public writer produces gzip framing, while ZIP entries require a raw DEFLATE stream.

[`hashzip/deflatewriter.go`](../hashzip/deflatewriter.go) adapts pgzip's parallel compression machinery to raw DEFLATE, omitting the gzip header and trailer. Large individual layers can be compressed across multiple CPU cores while the resulting entries remain readable by standard ZIP tools.

A regular DEFLATE writer could produce compatible archives, but would change the performance characteristics of compressing large individual layers. Performance changes should include measurements of elapsed time, output size, memory use, compression level, and CPU count on representative images. No compression-ratio or throughput advantage is promised for every workload.

## Compatibility boundaries

- The workflow has been checked with Docker Engine 29.8.2 using the classic image store (`overlay2`), including create, copy, update, restore, and pull from the embedded registry.
- Unit tests exercise both legacy and `blobs/sha256/` file layouts. They run without Docker.
- Multi-platform image indexes, arbitrary OCI layouts, signatures, attestations, and containerd image-store workflows are not currently covered by an end-to-end compatibility guarantee.
- A `.diz` ZIP is consumed by diz. It is not directly accepted by `docker load` or an OCI registry importer.
- Existing output files are truncated when creating or updating an archive. Keep input and output paths distinct.

Compatibility reports with a Docker version, image-store backend, and public reproduction image are useful contributions.
