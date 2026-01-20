{
  description = "Go Music DL - 一个完整的、工程化的 Go 音乐下载项目";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.gomod2nix.url = "github:nix-community/gomod2nix";
  inputs.gomod2nix.inputs.nixpkgs.follows = "nixpkgs";
  inputs.gomod2nix.inputs.flake-utils.follows = "flake-utils";

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    gomod2nix,
    ...
  } @ inputs: let
    allSystems = flake-utils.lib.allSystems;
  in flake-utils.lib.eachSystem allSystems (system: let
    # 导入 nixpkgs 并覆盖 Go 1.25 为默认版本
    pkgs = import nixpkgs {
      inherit system;
      overlays = [
        (final: prev: {
          go = prev.go_1_25;  # 全局默认 Go 1.25
          go_1_25 = prev.go_1_25.overrideAttrs (old: {
            patches = old.patches or [];
          });
        })
      ];
    };

    # macOS SDK 兼容处理
    callPackage = if pkgs.system == "aarch64-darwin" || pkgs.system == "x86_64-darwin"
      then pkgs.darwin.apple_sdk_11_0.callPackage
      else pkgs.callPackage;

    # gomod2nix 相关工具
    gomod2nixPkgs = gomod2nix.legacyPackages.${system};

    # 定义开发环境（替代 shell.nix）
    devShell = pkgs.mkShell {
      # 开发依赖（和之前 shell.nix 一致）
      buildInputs = [
        pkgs.go
        gomod2nixPkgs.gomod2nix
        pkgs.git
        pkgs.bash
        pkgs.cacert
      ];

      # 环境变量：强制 Go 1.25 版本
      env = {
        GOVERSION = "1.25.1";
        GOTOOLCHAIN = "local";
        GOPATH = "${builtins.getEnv "HOME"}/go";
        PATH = "${pkgs.go}/bin:${gomod2nixPkgs.gomod2nix}/bin:${builtins.getEnv "PATH"}";
      };

      # 进入开发环境的提示
      shellHook = ''
        echo "✅ 已加载 Go 1.25 开发环境（当前版本: $(go version)）"
        echo "📌 执行 gomod2nix generate 生成依赖配置文件"
        echo "📌 执行 nix build .#go-music-dl 构建项目"
      '';
    };
  in {
    # 项目包构建
    packages = rec {
      go-music-dl = (callPackage ./. ({
        buildGoApplication = gomod2nixPkgs.buildGoApplication;
        go = pkgs.go;
      })).overrideAttrs (oldAttrs: {
        doCheck = false;
        GOVERSION = "1.25.1";
        GOTOOLCHAIN = "local";
      });

      default = go-music-dl;

      # Docker 镜像构建
      docker_builder = pkgs.dockerTools.buildLayeredImage {
        name = "go-music-dl";
        tag = "latest";
        contents = [
          self.packages.${system}.go-music-dl
          pkgs.cacert
          pkgs.bash
        ];
        entrypoint = ["/bin/go-music-dl"];
      };
    };

    # 暴露开发环境（核心：devShells.default）
    devShells = {
      default = devShell;
    };

    # 代码格式化工具
    formatter = pkgs.alejandra;
  });
}