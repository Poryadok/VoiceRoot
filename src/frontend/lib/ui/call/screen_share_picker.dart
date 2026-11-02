import 'package:flutter/material.dart';

import '../../backend/screen_share_sources.dart';
import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';

class ScreenSharePickerResult {
  const ScreenSharePickerResult({
    required this.sourceId,
    required this.maxFrameRate,
    required this.captureSystemAudio,
  });
  final String? sourceId;
  final double maxFrameRate;
  final bool captureSystemAudio;
}

/// Browser capture chooses its source in the OS/browser permission dialog.
Future<ScreenSharePickerResult?> showScreenSharePicker(
  BuildContext context, {
  required List<ScreenShareSource> sources,
  required bool captureSystemAudioSupported,
  bool useSystemPicker = false,
}) => showDialog<ScreenSharePickerResult>(
  context: context,
  builder: (_) => ScreenSharePicker(
    sources: sources,
    captureSystemAudioSupported: captureSystemAudioSupported,
    useSystemPicker: useSystemPicker,
  ),
);

class ScreenSharePicker extends StatefulWidget {
  const ScreenSharePicker({
    super.key,
    required this.sources,
    required this.captureSystemAudioSupported,
    this.useSystemPicker = false,
  });
  final List<ScreenShareSource> sources;
  final bool captureSystemAudioSupported;
  final bool useSystemPicker;

  @override
  State<ScreenSharePicker> createState() => _ScreenSharePickerState();
}

class _ScreenSharePickerState extends State<ScreenSharePicker> {
  ScreenShareSourceType _type = ScreenShareSourceType.screen;
  String? _selected;
  double _fps = 30;
  bool _audio = false;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final colors = VoiceColors.of(context);
    final ru = Localizations.localeOf(context).languageCode == 'ru';
    String text(String en, String russian) => ru ? russian : en;
    final available = widget.sources.where((source) => source.type == _type);
    return Dialog(
      insetPadding: const EdgeInsets.all(16),
      backgroundColor: colors.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(6),
        side: BorderSide(color: colors.borderDefault),
      ),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 680),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(24, 12, 12, 12),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        l10n.screenShareStart,
                        style: Theme.of(context).textTheme.titleLarge,
                      ),
                    ),
                    IconButton(
                      tooltip: MaterialLocalizations.of(
                        context,
                      ).closeButtonTooltip,
                      onPressed: () => Navigator.pop(context),
                      icon: const Icon(Icons.close),
                    ),
                  ],
                ),
              ),
              const Divider(height: 1),
              if (!widget.useSystemPicker)
                Padding(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 16,
                    vertical: 8,
                  ),
                  child: Wrap(
                    spacing: 8,
                    children: [
                      for (final type in ScreenShareSourceType.values)
                        ChoiceChip(
                          label: Text(
                            type == ScreenShareSourceType.screen
                                ? l10n.screenShareEntireScreen
                                : l10n.screenShareWindow,
                          ),
                          selected: _type == type,
                          onSelected: (_) => setState(() {
                            _type = type;
                            _selected = null;
                          }),
                        ),
                    ],
                  ),
                ),
              Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    if (widget.useSystemPicker)
                      Container(
                        padding: const EdgeInsets.all(20),
                        decoration: BoxDecoration(
                          color: colors.muted,
                          borderRadius: BorderRadius.circular(6),
                        ),
                        child: Column(
                          children: [
                            const Icon(Icons.screen_share_outlined, size: 40),
                            const SizedBox(height: 12),
                            Text(
                              text(
                                'Choose a screen, window or browser tab in the system dialog.',
                                'Выберите экран, окно или вкладку браузера в системном диалоге.',
                              ),
                              textAlign: TextAlign.center,
                            ),
                          ],
                        ),
                      )
                    else if (available.isEmpty)
                      Padding(
                        padding: const EdgeInsets.symmetric(vertical: 24),
                        child: Text(
                          l10n.screenShareSourceUnavailable,
                          textAlign: TextAlign.center,
                        ),
                      )
                    else
                      LayoutBuilder(
                        builder: (context, constraints) {
                          final width = constraints.maxWidth < 450
                              ? constraints.maxWidth
                              : (constraints.maxWidth - 24) / 3;
                          return Wrap(
                            spacing: 12,
                            runSpacing: 12,
                            children: [
                              for (final source in available)
                                SizedBox(
                                  width: width,
                                  child: Semantics(
                                    selected: _selected == source.id,
                                    child: OutlinedButton(
                                      key: Key(
                                        'screen_share_source_${source.id}',
                                      ),
                                      style: OutlinedButton.styleFrom(
                                        padding: const EdgeInsets.all(8),
                                        side: BorderSide(
                                          color: _selected == source.id
                                              ? colors.profileAccent
                                              : colors.borderDefault,
                                          width: _selected == source.id ? 2 : 1,
                                        ),
                                        shape: RoundedRectangleBorder(
                                          borderRadius: BorderRadius.circular(
                                            6,
                                          ),
                                        ),
                                      ),
                                      onPressed: () =>
                                          setState(() => _selected = source.id),
                                      child: Column(
                                        children: [
                                          Container(
                                            height: 76,
                                            width: double.infinity,
                                            decoration: BoxDecoration(
                                              color: colors.muted,
                                              borderRadius:
                                                  BorderRadius.circular(4),
                                            ),
                                            child: Icon(
                                              source.type ==
                                                      ScreenShareSourceType
                                                          .screen
                                                  ? Icons.desktop_windows
                                                  : Icons.web_asset,
                                              color: colors.textSecondary,
                                              size: 32,
                                            ),
                                          ),
                                          const SizedBox(height: 8),
                                          Text(
                                            source.name,
                                            maxLines: 2,
                                            overflow: TextOverflow.ellipsis,
                                          ),
                                        ],
                                      ),
                                    ),
                                  ),
                                ),
                            ],
                          );
                        },
                      ),
                    const SizedBox(height: 16),
                    Tooltip(
                      message: widget.captureSystemAudioSupported
                          ? l10n.screenShareSystemAudio
                          : l10n.platformWebSystemAudioUnavailable,
                      child: SwitchListTile(
                        contentPadding: EdgeInsets.zero,
                        title: Text(l10n.screenShareSystemAudio),
                        subtitle: widget.captureSystemAudioSupported
                            ? null
                            : Text(l10n.platformWebSystemAudioUnavailable),
                        value: _audio,
                        onChanged: widget.captureSystemAudioSupported
                            ? (value) => setState(() => _audio = value)
                            : null,
                      ),
                    ),
                    const SizedBox(height: 12),
                    Wrap(
                      spacing: 16,
                      runSpacing: 12,
                      children: [
                        SizedBox(
                          width: 170,
                          child: DropdownButtonFormField<double>(
                            initialValue: _fps,
                            decoration: InputDecoration(
                              labelText: text('Frame rate', 'Частота кадров'),
                              border: const OutlineInputBorder(),
                            ),
                            items: const [
                              DropdownMenuItem(
                                value: 15,
                                child: Text('15 FPS'),
                              ),
                              DropdownMenuItem(
                                value: 30,
                                child: Text('30 FPS'),
                              ),
                            ],
                            onChanged: (value) {
                              if (value != null) setState(() => _fps = value);
                            },
                          ),
                        ),
                        SizedBox(
                          width: 170,
                          child: InputDecorator(
                            decoration: InputDecoration(
                              labelText: text('Resolution', 'Разрешение'),
                              border: const OutlineInputBorder(),
                            ),
                            child: Text(text('Automatic', 'Автоматически')),
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              const Divider(height: 1),
              Padding(
                padding: const EdgeInsets.all(16),
                child: Wrap(
                  alignment: WrapAlignment.end,
                  spacing: 12,
                  runSpacing: 8,
                  children: [
                    TextButton(
                      onPressed: () => Navigator.pop(context),
                      child: Text(
                        MaterialLocalizations.of(context).cancelButtonLabel,
                      ),
                    ),
                    FilledButton(
                      key: const Key('screen_share_picker_start'),
                      onPressed: widget.useSystemPicker || _selected != null
                          ? () => Navigator.pop(
                              context,
                              ScreenSharePickerResult(
                                sourceId: _selected,
                                maxFrameRate: _fps,
                                captureSystemAudio: _audio,
                              ),
                            )
                          : null,
                      child: Text(l10n.screenShareStart),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
