import 'dart:math' as math;
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';

/// A local photo editor. Its result is flattened PNG bytes for File upload.
class StoryPhotoEditor extends StatefulWidget {
  const StoryPhotoEditor({super.key, required this.source});

  final Uint8List source;

  static const canvasKey = Key('story_photo_canvas');
  static const grayscaleKey = Key('story_photo_grayscale');
  static const stickerKey = Key('story_photo_sticker');
  static const originalKey = Key('story_photo_original');
  static const cancelKey = Key('story_photo_cancel');
  static const applyKey = Key('story_photo_apply');

  @override
  State<StoryPhotoEditor> createState() => _StoryPhotoEditorState();
}

enum _PhotoFilter { none, grayscale, warm }

class _StoryPhotoEditorState extends State<StoryPhotoEditor> {
  final _boundary = GlobalKey();
  final _strokes = <List<Offset>>[];
  double? _aspectRatio;
  int? _sourceLongestSide;
  ui.Image? _sourceImage;
  _PhotoFilter _filter = _PhotoFilter.none;
  bool _sticker = false;
  bool _exporting = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _readDimensions();
  }

  Future<void> _readDimensions() async {
    ui.Codec? codec;
    ui.Image? decoded;
    try {
      codec = await ui.instantiateImageCodec(widget.source);
      final frame = await codec.getNextFrame();
      decoded = frame.image;
      final ratio = decoded.width / decoded.height;
      final longestSide = math.max(decoded.width, decoded.height);
      if (mounted) {
        setState(() {
          _aspectRatio = ratio;
          _sourceLongestSide = longestSide;
          _sourceImage = decoded;
        });
        decoded = null; // The widget now owns the decoded image.
      }
    } on Object {
      if (mounted) {
        setState(
          () => _error = _label(
            'Could not open photo',
            'Не удалось открыть фото',
          ),
        );
      }
    } finally {
      decoded?.dispose();
      codec?.dispose();
    }
  }

  @override
  void dispose() {
    _sourceImage?.dispose();
    super.dispose();
  }

  String _label(String en, String ru) =>
      Localizations.localeOf(context).languageCode == 'ru' ? ru : en;

  List<double> get _filterMatrix => switch (_filter) {
    _PhotoFilter.none => const [
      1,
      0,
      0,
      0,
      0,
      0,
      1,
      0,
      0,
      0,
      0,
      0,
      1,
      0,
      0,
      0,
      0,
      0,
      1,
      0,
    ],
    _PhotoFilter.grayscale => const [
      .2126,
      .7152,
      .0722,
      0,
      0,
      .2126,
      .7152,
      .0722,
      0,
      0,
      .2126,
      .7152,
      .0722,
      0,
      0,
      0,
      0,
      0,
      1,
      0,
    ],
    _PhotoFilter.warm => const [
      1.08,
      0,
      0,
      0,
      4,
      0,
      1.02,
      0,
      0,
      0,
      0,
      0,
      .88,
      0,
      0,
      0,
      0,
      0,
      1,
      0,
    ],
  };

  void _startStroke(DragStartDetails details, Size size) {
    setState(() => _strokes.add([_point(details.localPosition, size)]));
  }

  void _extendStroke(DragUpdateDetails details, Size size) {
    if (_strokes.isEmpty) return;
    setState(() => _strokes.last.add(_point(details.localPosition, size)));
  }

  Offset _point(Offset local, Size size) => Offset(
    (local.dx / size.width).clamp(0, 1),
    (local.dy / size.height).clamp(0, 1),
  );

  Future<void> _apply() async {
    setState(() {
      _exporting = true;
      _error = null;
    });
    try {
      // Wait for the loading state to paint before capturing the boundary.
      await WidgetsBinding.instance.endOfFrame;
      final boundary =
          _boundary.currentContext!.findRenderObject() as RenderRepaintBoundary;
      final longest = math.max(boundary.size.width, boundary.size.height);
      final targetLongest = math.max(720, math.min(2048, _sourceLongestSide!));
      ui.Image? image;
      ByteData? data;
      try {
        image = await boundary.toImage(pixelRatio: targetLongest / longest);
        data = await image.toByteData(format: ui.ImageByteFormat.png);
      } finally {
        image?.dispose();
      }
      if (data == null) throw StateError('PNG encoding failed');
      if (mounted) Navigator.of(context).pop(data.buffer.asUint8List());
    } on Object {
      if (mounted) {
        setState(() {
          _exporting = false;
          _error = _label('Could not save photo', 'Не удалось сохранить фото');
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final ratio = _aspectRatio;
    return Scaffold(
      appBar: AppBar(title: Text(_label('Edit photo', 'Редактировать фото'))),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            children: [
              Expanded(
                child: Center(
                  child: ratio == null
                      ? _error == null
                            ? const CircularProgressIndicator()
                            : Text(_error!)
                      : LayoutBuilder(
                          builder: (context, constraints) {
                            final width = math.min(
                              constraints.maxWidth,
                              math.min(420.0, constraints.maxHeight * ratio),
                            );
                            final size = Size(width, width / ratio);
                            return SizedBox(
                              width: size.width,
                              height: size.height,
                              child: GestureDetector(
                                key: StoryPhotoEditor.canvasKey,
                                onPanStart: _exporting
                                    ? null
                                    : (details) => _startStroke(details, size),
                                onPanUpdate: _exporting
                                    ? null
                                    : (details) => _extendStroke(details, size),
                                child: RepaintBoundary(
                                  key: _boundary,
                                  child: Stack(
                                    fit: StackFit.expand,
                                    children: [
                                      ColorFiltered(
                                        colorFilter: ColorFilter.matrix(
                                          _filterMatrix,
                                        ),
                                        child: RawImage(
                                          image: _sourceImage,
                                          fit: BoxFit.fill,
                                        ),
                                      ),
                                      CustomPaint(
                                        painter: _DoodlePainter(_strokes),
                                      ),
                                      if (_sticker)
                                        const Center(
                                          child: Text(
                                            '⭐',
                                            style: TextStyle(fontSize: 48),
                                          ),
                                        ),
                                    ],
                                  ),
                                ),
                              ),
                            );
                          },
                        ),
                ),
              ),
              if (ratio != null) ...[
                Wrap(
                  spacing: 8,
                  children: [
                    ChoiceChip(
                      label: Text(_label('Normal', 'Обычный')),
                      selected: _filter == _PhotoFilter.none,
                      onSelected: _exporting
                          ? null
                          : (_) => setState(() => _filter = _PhotoFilter.none),
                    ),
                    ChoiceChip(
                      key: StoryPhotoEditor.grayscaleKey,
                      label: Text(_label('Mono', 'Ч/б')),
                      selected: _filter == _PhotoFilter.grayscale,
                      onSelected: _exporting
                          ? null
                          : (_) => setState(
                              () => _filter = _PhotoFilter.grayscale,
                            ),
                    ),
                    ChoiceChip(
                      label: Text(_label('Warm', 'Тёплый')),
                      selected: _filter == _PhotoFilter.warm,
                      onSelected: _exporting
                          ? null
                          : (_) => setState(() => _filter = _PhotoFilter.warm),
                    ),
                    FilterChip(
                      key: StoryPhotoEditor.stickerKey,
                      label: Text(_label('Star sticker', 'Стикер звезда')),
                      selected: _sticker,
                      onSelected: _exporting
                          ? null
                          : (value) => setState(() => _sticker = value),
                    ),
                  ],
                ),
                const SizedBox(height: 8),
                Text(
                  _label(
                    'Draw on the photo with your finger',
                    'Рисуйте по фото пальцем',
                  ),
                ),
                if (_strokes.isNotEmpty)
                  TextButton(
                    onPressed: _exporting
                        ? null
                        : () => setState(_strokes.clear),
                    child: Text(_label('Clear drawing', 'Очистить рисунок')),
                  ),
              ],
              if (_error != null && ratio != null)
                Text(
                  _error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                ),
              if (_exporting) const LinearProgressIndicator(),
              Row(
                children: [
                  TextButton(
                    key: StoryPhotoEditor.cancelKey,
                    onPressed: _exporting
                        ? null
                        : () => Navigator.of(context).pop(),
                    child: Text(_label('Cancel', 'Отмена')),
                  ),
                  const Spacer(),
                  TextButton(
                    key: StoryPhotoEditor.originalKey,
                    onPressed: _exporting || ratio == null
                        ? null
                        : () => Navigator.of(context).pop(widget.source),
                    child: Text(_label('Original', 'Оригинал')),
                  ),
                  FilledButton(
                    key: StoryPhotoEditor.applyKey,
                    onPressed: _exporting || ratio == null ? null : _apply,
                    child: Text(_label('Apply', 'Применить')),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _DoodlePainter extends CustomPainter {
  const _DoodlePainter(this.strokes);

  final List<List<Offset>> strokes;

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = Colors.white
      ..strokeWidth = 4
      ..strokeCap = StrokeCap.round
      ..strokeJoin = StrokeJoin.round
      ..style = PaintingStyle.stroke;
    for (final stroke in strokes) {
      if (stroke.isEmpty) continue;
      final points = stroke
          .map((p) => Offset(p.dx * size.width, p.dy * size.height))
          .toList();
      if (points.length == 1) {
        canvas.drawCircle(points.first, 2, paint..style = PaintingStyle.fill);
        paint.style = PaintingStyle.stroke;
        continue;
      }
      final path = Path()..moveTo(points.first.dx, points.first.dy);
      for (final point in points.skip(1)) {
        path.lineTo(point.dx, point.dy);
      }
      canvas.drawPath(path, paint);
    }
  }

  @override
  bool shouldRepaint(_DoodlePainter oldDelegate) => true;
}
