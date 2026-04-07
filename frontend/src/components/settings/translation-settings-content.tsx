import { useState, useEffect } from "react";
import { toast } from "sonner";
import {
  Settings,
  Key,
  Sparkles,
  Loader2,
  AlertCircle,
  CheckCircle2,
} from "lucide-react";
import {
  Button,
} from "@/components/ui/button";
import {
  Input,
} from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { useTranslationSettings, useTranslationModels, useUpdateTranslationSettings } from "@/queries/items";
import { useI18n } from "@/lib/i18n";

interface TranslationSettingsContentProps {
  onOpenTranslationHelp?: () => void;
}

export function TranslationSettingsContent({ onOpenTranslationHelp }: TranslationSettingsContentProps) {
  const { t } = useI18n();
  // Separate state for model and target language
  const [translationModel, setTranslationModel] = useState("");
  const [translationTargetLanguage, setTranslationTargetLanguage] = useState("ru");
  const [autoTranslateMode, setAutoTranslateMode] = useState(false);
  const [openaiAPIKey, setOpenaiAPIKey] = useState("");

  // Fetch current settings
  const { data: settings, isLoading: settingsLoading, error: settingsError } =
    useTranslationSettings();

  // Fetch available models
  const { data: models, isLoading: modelsLoading, error: modelsError, refetch: refetchModels } =
    useTranslationModels();

  // Fetch models on mount
  useEffect(() => {
    void refetchModels();
  }, [refetchModels]);

  // Update settings mutation
  const updateSettings = useUpdateTranslationSettings();

  const handleUpdateSettings = () => {
    const updatePayload: { translation_model?: string; translation_target_language?: string; openai_api_key?: string; auto_translate_mode?: boolean } = {};

    if (translationTargetLanguage !== settings?.translation_target_language) {
      updatePayload.translation_target_language = translationTargetLanguage;
    }

    if (translationModel && translationModel !== settings?.translation_model) {
      updatePayload.translation_model = translationModel;
    }

    if (openaiAPIKey.trim()) {
      updatePayload.openai_api_key = openaiAPIKey.trim();
    }

    if (autoTranslateMode !== settings?.auto_translate_mode) {
      updatePayload.auto_translate_mode = autoTranslateMode;
    }

    if (Object.keys(updatePayload).length === 0) {
      // No changes to save
      toast.info(t("settings.translation.noChanges"));
      return;
    }

    updateSettings.mutate(updatePayload as any, {
      onSuccess: () => {
        toast.success(t("settings.translation.settingsUpdated"));
        setOpenaiAPIKey("");
      },
      onError: (_error) => {
        toast.error(t("settings.translation.updateFailed"));
      },
    });
  };

  const handleFetchModels = async () => {
    try {
      await refetchModels();
      toast.success(t("settings.translation.modelsFetched"));
    } catch (error) {
      toast.error(t("settings.translation.fetchFailed"));
    }
  };

  // Extract available target languages (simplified - using known supported languages)
  const supportedTargetLanguages = ["ru", "en", "zh", "de", "fr", "es", "pt", "sv", "ja", "ko"];

  // Handle loading state
  if (settingsLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="flex items-center gap-2 text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          <span className="text-sm">{t("settings.translation.loading")}</span>
        </div>
      </div>
    );
  }

  // Handle error state
  if (settingsError) {
    return (
      <div className="flex h-full items-center justify-center gap-2 text-red-500">
        <AlertCircle className="h-4 w-4" />
        <span className="text-sm">{t("settings.translation.error")}</span>
      </div>
    );
  }

  // Initialize state from settings when loaded
  useEffect(() => {
    if (settings) {
      setTranslationTargetLanguage(settings.translation_target_language || "ru");
      setTranslationModel(settings.translation_model || "");
      setAutoTranslateMode(settings.auto_translate_mode || false);
    }
  }, [settings]);

  return (
    <div className="space-y-6">
      {/* API Key Status Section */}
      <div className="rounded-lg border border-border bg-card p-4 space-y-3">
        <div className="flex items-start gap-3">
          <div className="mt-0.5">
            <Key className="h-4 w-4 text-muted-foreground" />
          </div>
          <div className="flex-1 space-y-2">
            <p className="text-sm font-medium">{t("settings.translation.apiKey.status")}</p>
            <div className="flex items-center gap-2 text-xs">
              {settings?.has_api_key ? (
                <>
                  <CheckCircle2 className="h-3.5 w-3.5 text-green-500" />
                  <span className="text-muted-foreground">{t("settings.translation.apiKey.configured")}</span>
                  <span className="text-muted-foreground/70">
                    ({settings.api_key_source === "env" ? t("settings.translation.apiKey.sourceEnv") : t("settings.translation.apiKey.sourceDb")})
                  </span>
                </>
              ) : (
                <>
                  <AlertCircle className="h-3.5 w-3.5 text-yellow-500" />
                  <span className="text-muted-foreground">{t("settings.translation.apiKey.unconfigured")}</span>
                </>
              )}
            </div>
          </div>
        </div>

        {/* API Key Input Section */}
        <div className="mt-4 pt-4 border-t border-border/50 space-y-3">
          {settings?.api_key_source === "env" ? (
            <div className="space-y-1.5">
              <Input
                value={settings.masked_api_key}
                readOnly
                disabled
                className="bg-muted/50 font-mono text-xs"
              />
              <p className="text-[11px] text-muted-foreground">
                {t("settings.translation.apiKey.sourceEnv")}
              </p>
            </div>
          ) : (
            <div className="space-y-1.5">
              <Input
                type="password"
                value={openaiAPIKey}
                onChange={(e) => setOpenaiAPIKey(e.target.value)}
                placeholder={settings?.has_api_key ? t("settings.translation.apiKey.placeholderUpdate") : t("settings.translation.apiKey.placeholder")}
                disabled={updateSettings.isPending}
                className="font-mono text-xs"
              />
              {settings?.has_api_key && (
                <p className="text-[11px] text-muted-foreground">
                  {t("settings.translation.apiKey.masked")}: {settings.masked_api_key}
                </p>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Translation Model Section */}
      <div className="space-y-3">
        <div className="flex items-start justify-between">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <Sparkles className="h-4 w-4" />
              <p className="text-sm font-medium">{t("settings.translation.model.label")}</p>
            </div>
            <p className="text-[13px] text-muted-foreground">
              {t("settings.translation.model.description")}
            </p>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={handleFetchModels}
            disabled={modelsLoading || updateSettings.isPending}
          >
            {modelsLoading ? (
              <>
                <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                {t("settings.translation.model.fetching")}
              </>
            ) : (
              <>
                <Sparkles className="mr-2 h-3.5 w-3.5" />
                {t("settings.translation.model.fetch")}
              </>
            )}
          </Button>
        </div>

        <div>
          {models && models.models.length > 0 ? (
            <Select value={translationModel} onValueChange={setTranslationModel} disabled={updateSettings.isPending}>
              <SelectTrigger>
                <SelectValue placeholder={t("settings.translation.selectModel")} />
              </SelectTrigger>
              <SelectContent>
                {models.models.map((model) => (
                  <SelectItem key={model.id} value={model.id}>
                    {model.id}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <Input
              value={translationModel}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                setTranslationModel(e.target.value);
              }}
              placeholder={t("settings.translation.model.manualPlaceholder")}
              disabled={updateSettings.isPending}
            />
          )}
        </div>

        {modelsError && (
          <p className="text-xs text-red-500">{t("settings.translation.fetchFailed")}</p>
        )}
      </div>

      {/* Target Language Section */}
      <div className="space-y-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <Settings className="h-4 w-4" />
            <p className="text-sm font-medium">{t("settings.translation.targetLanguage.label")}</p>
          </div>
          <p className="text-[13px] text-muted-foreground">
            {t("settings.translation.targetLanguage.description")}
          </p>
        </div>

        <Select
          value={translationTargetLanguage}
          onValueChange={(value: string) => setTranslationTargetLanguage(value)}
          disabled={updateSettings.isPending}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {supportedTargetLanguages.map((lang) => (
              <SelectItem key={lang} value={lang}>
                {lang.toUpperCase()}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Auto-translate Mode Section */}
      <div className="flex items-center justify-between rounded-lg border border-border bg-card p-4">
        <div className="space-y-0.5">
          <p className="text-sm font-medium">{t("settings.translation.autoMode.label")}</p>
          <p className="text-[13px] text-muted-foreground">
            {t("settings.translation.autoMode.description")}
          </p>
        </div>
        <Switch
          checked={autoTranslateMode}
          onCheckedChange={setAutoTranslateMode}
          disabled={updateSettings.isPending || !settings?.has_api_key}
        />
      </div>

      {/* Update Button */}
      <Button
        onClick={handleUpdateSettings}
        disabled={updateSettings.isPending}
        className="w-full"
      >
        {updateSettings.isPending ? (
          <>
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            {t("settings.translation.updating")}
          </>
        ) : (
          <>
            <Settings className="mr-2 h-4 w-4" />
            {t("settings.translation.saveChanges")}
          </>
        )}
      </Button>

      {/* Help Link */}
      {onOpenTranslationHelp && (
        <button
          onClick={onOpenTranslationHelp}
          className="mt-4 text-xs text-muted-foreground hover:text-foreground underline"
        >
          {t("settings.translation.learnMore")}
        </button>
      )}
    </div>
  );
}
