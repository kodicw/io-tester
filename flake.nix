{
  description = "io-tester — filesystem I/O benchmarks for dev workloads";
  nixConfig.bash-prompt = "\[io-tester\]$ ";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };

        # ── The Go I/O benchmark tool ──────────────────────────
        io-tester = pkgs.buildGoModule {
          pname = "io-tester";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-Wn8K/t9e7zoLPyW4JInLGiT7m3yp3ZlY5K5JiYz8sCQ=";

          nativeBuildInputs = [ pkgs.makeWrapper ];
          buildInputs = [ pkgs.fastfetch ];

          ldflags = [ "-s" "-w" ];

          postInstall = ''
            wrapProgram $out/bin/io-tester \
              --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.fastfetch pkgs.tinycc ]}
          '';

          # Allow the build to use a user-supplied compiler (CC) but provide a
          # lightweight default (tcc) so the build_* benchmarks run out of the box.

          meta = with pkgs.lib; {
            description = "Dev-workload performance benchmark: filesystem I/O, compilation, and process overhead";
            homepage = "https://github.com/charles/io-tester";
            license = licenses.mit;
            platforms = platforms.unix;
            mainProgram = "io-tester";
          };
        };

        # ── Dev-workload wrapper scripts for external tools ────
        run-fs-mark = pkgs.writeShellScriptBin "io-fs-mark" ''
          set -e
          DIR=''${1:-$(mktemp -d /tmp/io-tester-fsmark-XXXX)}
          FILES=''${IO_FILES:-10000}
          SIZE=''${IO_SIZE:-256}
          DEPTH=''${IO_DEPTH:-4}
          ITERS=''${IO_ITERS:-1}

          echo "╔═══ fs_mark — synchronous small-file benchmark ═══╗"
          echo "  files: $FILES  size: ''${SIZE}B  depth: $DEPTH  iters: $ITERS"
          echo "  directory: $DIR"
          echo ""

          mkdir -p "$DIR"
          # fs_mark: -d directory -n files per iteration -s file size -D subdirs -L iterations -t threads -S sync method
          # Use sync method 1 (fsync before close) for realistic dev workload
          ${pkgs.fsmark}/bin/fs_mark \
            -d "$DIR" \
            -n "$FILES" \
            -s "$SIZE" \
            -D "$DEPTH" \
            -L "$ITERS" \
            -t 1 \
            -S 1 \
            -k

          echo ""
          echo "Done. Results in: $DIR"
        '';

        run-fio = pkgs.writeShellScriptBin "io-fio" ''
          set -e
          DIR=''${1:-$(mktemp -d /tmp/io-tester-fio-XXXX)}
          SIZE=''${IO_SIZE:-256}
          FILES=''${IO_FILES:-10000}

          echo "╔═══ fio — dev workload small-file bench ═══╗"
          echo "  files: $FILES  size: ''${SIZE}B"
          echo ""

          mkdir -p "$DIR"
          # Create a fio job that mimics dev work: many small random writes + reads
          ${pkgs.fio}/bin/fio --directory="$DIR" --name=io-tester \
            --ioengine=sync \
            --size=''${IO_SIZE_BATCH:-$((FILES * SIZE / 1024 / 1024))}M \
            --rw=randrw \
            --rwmixwrite=50 \
            --bs="$SIZE" \
            --nrfiles="$FILES" \
            --openfiles=100 \
            --file_service_type=roundrobin \
            --group_reporting=1 \
            --fallocate=none \
            --runtime=''${IO_RUNTIME:-10}

          echo ""
          echo "Done."
        '';

        run-bonnie = pkgs.writeShellScriptBin "io-bonnie" ''
          set -e
          DIR=''${1:-$(mktemp -d /tmp/io-tester-bonnie-XXXX)}
          SIZE=''${IO_SIZE:-256}

          echo "╔═══ bonnie++ — filesystem benchmark ═══╗"
          echo "  size: ''${SIZE}B"
          echo ""

          mkdir -p "$DIR"
          # bonnie++ runs sequential/random I/O with file create/stat/delete tests
          ${pkgs.bonnie}/bin/bonnie++ \
            -d "$DIR" \
            -s ''${IO_BONNIE_SIZE:-256} \
            -n ''${IO_BONNIE_FILES:-5000} \
            -r 0 \
            -u $(whoami 2>/dev/null || echo nobody)

          echo ""
          echo "Done."
        '';

        # ── Unified runner ─────────────────────────────────────
        runner = pkgs.writeShellScriptBin "io-tester-runner" ''
          set -e

          die() { echo "$@" >&2; exit 1; }
          usage() {
            echo "io-tester — dev I/O benchmarks"
            echo ""
            echo "Usage:  nix run . [subcommand|benchmark-name] [args...]"
            echo ""
            echo "Subcommands:"
            echo "  go           Run Go benchmarks (default)"
            echo "  fs_mark      Run fs_mark (synchronous small file creation)"
            echo "  fio          Run fio (flexible I/O tester)"
            echo "  bonnie       Run bonnie++ (filesystem benchmark)"
            echo "  all-tools    Run all external tools (fs_mark + fio + bonnie)"
            echo "  help         Show this help"
            echo ""
            echo "Examples:"
            echo "  nix run .                          # Go benchmarks (all)"
            echo "  nix run . -- small_write           # specific Go bench"
            echo "  nix run .#fs_mark                  # fs_mark with defaults"
            echo "  nix run .#all-tools                # all external tools"
            echo "  IO_FILES=20000 nix run .#fs_mark    # 20k files"
          }

          ARG1=''${1:-}

          case "$ARG1" in
            fs_mark|fsmark)
              shift
              exec ${run-fs-mark}/bin/io-fs-mark "$@"
              ;;
            fio)
              shift
              exec ${run-fio}/bin/io-fio "$@"
              ;;
            bonnie|bonnie++)
              shift
              exec ${run-bonnie}/bin/io-bonnie "$@"
              ;;
            all-tools|all_external)
              shift
              echo "═══ Running all external tools ═══"
              echo ""
              echo "--- fs_mark ---"
              ${run-fs-mark}/bin/io-fs-mark "$@"
              echo ""
              echo "--- bonnie++ ---"
              ${run-bonnie}/bin/io-bonnie "$@"
              echo ""
              echo "--- fio ---"
              ${run-fio}/bin/io-fio "$@"
              echo ""
              echo "═══ All external tools complete ═══"
              ;;
            help)
              # Runner-level help (use 'nix run . help')
              usage
              echo ""
              echo "For Go benchmark details: nix run . -- --help"
              ;;
            *)
              # Default: pass everything through to the Go binary
              exec ${io-tester}/bin/io-tester "$@"
              ;;
          esac
        '';

      in {
        packages = {
          default = io-tester;
          io-tester = io-tester;
        };

        # Individual apps for each external tool
        apps = {
          default = {
            type = "app";
            program = "${runner}/bin/io-tester-runner";
          };
          go = {
            type = "app";
            program = "${io-tester}/bin/io-tester";
          };
          fs_mark = {
            type = "app";
            program = "${run-fs-mark}/bin/io-fs-mark";
          };
          fio = {
            type = "app";
            program = "${run-fio}/bin/io-fio";
          };
          bonnie = {
            type = "app";
            program = "${run-bonnie}/bin/io-bonnie";
          };
          all-tools = {
            type = "app";
            program = "${runner}/bin/io-tester-runner";
          };
        };

        devShells.default = pkgs.mkShell {
          name = "io-tester-dev";
          packages = with pkgs; [
            go
            gopls
            gotools
            fsmark
            fio
            bonnie
          ];
          shellHook = ''
            echo "io-tester dev shell"
            echo "  run:  nix run .  or  go run ."
            echo "  ext:  fs_mark, fio, bonnie++ available"
          '';
        };
      });
}
