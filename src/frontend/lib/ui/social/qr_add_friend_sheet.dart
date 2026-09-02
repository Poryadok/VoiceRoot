import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../../l10n/app_localizations.dart';
import '../../routing/deep_link_urls.dart';
import '../../state/profile_deep_link_resolver.dart';
import '../../state/social_providers.dart';
import 'profile_detail_sheet.dart';

/// QR share + scan/paste profile link to add friends (friends.md).
class QrAddFriendSheet extends ConsumerStatefulWidget {
  const QrAddFriendSheet({super.key});

  static const Key sheetKey = Key('qr_add_friend_sheet');
  static const Key myQrKey = Key('qr_add_friend_my_qr');
  static const Key scanFieldKey = Key('qr_add_friend_scan_field');
  static const Key scanSubmitKey = Key('qr_add_friend_scan_submit');

  static Future<void> show(BuildContext context) {
    return showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => UncontrolledProviderScope(
        container: ProviderScope.containerOf(context),
        child: const QrAddFriendSheet(),
      ),
    );
  }

  @override
  ConsumerState<QrAddFriendSheet> createState() => _QrAddFriendSheetState();
}

class _QrAddFriendSheetState extends ConsumerState<QrAddFriendSheet>
    with SingleTickerProviderStateMixin {
  late final TabController _tabs;
  final _scanController = TextEditingController();
  var _scanBusy = false;
  String? _scanError;

  @override
  void initState() {
    super.initState();
    _tabs = TabController(length: 2, vsync: this);
  }

  @override
  void dispose() {
    _tabs.dispose();
    _scanController.dispose();
    super.dispose();
  }

  Future<void> _copyProfileLink(String url) async {
    await Clipboard.setData(ClipboardData(text: url));
    if (!mounted) return;
    final l10n = AppLocalizations.of(context)!;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(l10n.socialQrLinkCopied)));
  }

  Future<void> _openScannedProfile() async {
    final raw = _scanController.text.trim();
    if (raw.isEmpty) return;
    setState(() {
      _scanBusy = true;
      _scanError = null;
    });
    final profileId = await resolveProfileIdFromDeepLinkText(ref, raw);
    if (!mounted) return;
    if (profileId == null) {
      setState(() {
        _scanBusy = false;
        _scanError = AppLocalizations.of(context)!.socialQrScanInvalid;
      });
      return;
    }
    setState(() => _scanBusy = false);
    Navigator.of(context).pop();
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => UncontrolledProviderScope(
        container: ProviderScope.containerOf(context),
        child: ProfileDetailSheet(profileId: profileId),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final profileAsync = ref.watch(activeProfileProvider);

    return SafeArea(
      child: Padding(
        padding: EdgeInsets.only(
          left: 16,
          right: 16,
          top: 12,
          bottom: MediaQuery.viewInsetsOf(context).bottom + 16,
        ),
        child: Column(
          key: QrAddFriendSheet.sheetKey,
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              l10n.socialQrAddFriendTitle,
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 8),
            TabBar(
              controller: _tabs,
              tabs: [
                Tab(text: l10n.socialQrTabMyCode),
                Tab(text: l10n.socialQrTabScan),
              ],
            ),
            const SizedBox(height: 16),
            SizedBox(
              height: 320,
              child: TabBarView(
                controller: _tabs,
                children: [
                  profileAsync.when(
                    loading: () => const Center(
                      child: CircularProgressIndicator(),
                    ),
                    error: (error, stackTrace) => Center(
                      child: Text(l10n.socialQrProfileUnavailable),
                    ),
                    data: (profile) {
                      final username = profile?.username.trim();
                      if (username == null || username.isEmpty) {
                        return Center(
                          child: Text(l10n.socialQrProfileUnavailable),
                        );
                      }
                      final url = profileShareUrl(username);
                      return Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          QrImageView(
                            key: QrAddFriendSheet.myQrKey,
                            data: url,
                            size: 200,
                            backgroundColor: Colors.white,
                          ),
                          const SizedBox(height: 12),
                          SelectableText('@$username'),
                          const SizedBox(height: 8),
                          TextButton.icon(
                            onPressed: () => _copyProfileLink(url),
                            icon: const Icon(Icons.copy_outlined),
                            label: Text(l10n.socialQrCopyLink),
                          ),
                        ],
                      );
                    },
                  ),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Text(
                        l10n.socialQrScanHint,
                        style: Theme.of(context).textTheme.bodyMedium,
                      ),
                      const SizedBox(height: 12),
                      TextField(
                        key: QrAddFriendSheet.scanFieldKey,
                        controller: _scanController,
                        decoration: InputDecoration(
                          hintText: l10n.socialQrScanPlaceholder,
                          isDense: true,
                        ),
                        minLines: 2,
                        maxLines: 3,
                        enabled: !_scanBusy,
                      ),
                      if (_scanError != null) ...[
                        const SizedBox(height: 8),
                        Text(
                          _scanError!,
                          style: TextStyle(
                            color: Theme.of(context).colorScheme.error,
                          ),
                        ),
                      ],
                      const SizedBox(height: 12),
                      FilledButton(
                        key: QrAddFriendSheet.scanSubmitKey,
                        onPressed: _scanBusy ? null : _openScannedProfile,
                        child: _scanBusy
                            ? const SizedBox(
                                width: 18,
                                height: 18,
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                ),
                              )
                            : Text(l10n.socialQrScanSubmit),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
