{
  lib,
  config,
  ...
}:
let
  cfg = config.programs.socialstats;
in
{
  options.programs.socialstats = {
    enable = lib.mkEnableOption "socialstats analytics collector (systemd user timer)";

    package = lib.mkOption {
      type = lib.types.package;
      description = "socialstats package to install and run.";
    };

    interval = lib.mkOption {
      type = lib.types.str;
      default = "daily";
      example = "*-*-* 06:30:00";
      description = ''
        systemd OnCalendar expression for the user timer. Default is daily.
        The service runs `socialstats fetch`, which pulls YouTube Analytics
        and imports any new X CSV exports found in the watch directory.
      '';
    };

    configDir = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      example = "/home/user/.config/socialstats";
      description = ''
        Override the config directory (OAuth client JSON, refresh token, state).
        Unset uses $XDG_CONFIG_HOME/socialstats.
      '';
    };

    youtubeChannelId = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      example = "UCexample000000000000000";
      description = ''
        YouTube channel id, passed to the service as SOCIALSTATS_CHANNEL_ID.
        null leaves detection to the authorized account (channels?mine=true).
      '';
    };

    xHandle = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      example = "example_handle";
      description = ''
        Optional X handle label, passed as SOCIALSTATS_X_HANDLE.
        null leaves the label unset. Imported CSV rows do not require it.
      '';
    };

    webhookUrl = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      description = ''
        Optional URL that receives a POST of latest.json after a successful run.
        Prefer webhookUrlFile if the URL contains a secret.
      '';
    };

    webhookUrlFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = null;
      description = ''
        File whose contents are the webhook URL, optionally as
        SOCIALSTATS_WEBHOOK_URL=https://...
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = !(cfg.webhookUrl != null && cfg.webhookUrlFile != null);
        message = "programs.socialstats: set webhookUrl or webhookUrlFile, not both";
      }
    ];

    home.packages = [ cfg.package ];

    systemd.user.services.socialstats = {
      Unit = {
        Description = "Collect YouTube and X creator analytics";
        After = [ "network-online.target" ];
      };
      Service = {
        Type = "oneshot";
        ExecStart = "${cfg.package}/bin/socialstats fetch";
        Environment =
          (lib.optional (cfg.configDir != null) "SOCIALSTATS_CONFIG_DIR=${cfg.configDir}")
          ++ (lib.optional (cfg.youtubeChannelId != null) "SOCIALSTATS_CHANNEL_ID=${cfg.youtubeChannelId}")
          ++ (lib.optional (cfg.xHandle != null) "SOCIALSTATS_X_HANDLE=${cfg.xHandle}")
          ++ (lib.optional (cfg.webhookUrl != null) "SOCIALSTATS_WEBHOOK_URL=${cfg.webhookUrl}")
          ++ (lib.optional (
            cfg.webhookUrlFile != null
          ) "SOCIALSTATS_WEBHOOK_URL_FILE=${toString cfg.webhookUrlFile}");
      };
    };

    systemd.user.timers.socialstats = {
      Unit.Description = "Collect YouTube and X creator analytics";
      Timer = {
        OnCalendar = cfg.interval;
        Persistent = true;
        RandomizedDelaySec = "10m";
      };
      Install.WantedBy = [ "timers.target" ];
    };
  };
}
