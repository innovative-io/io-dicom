#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavutil/avutil.h>
#include <libavutil/error.h>
#include <libavutil/frame.h>
#include <libavutil/imgutils.h>
#include <libswscale/swscale.h>

#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static void io_ffmpeg_set_error(char* err, size_t err_len, const char* msg) {
	if (!err || err_len == 0) {
		return;
	}
	if (!msg) {
		err[0] = '\0';
		return;
	}
	snprintf(err, err_len, "%s", msg);
}

static void io_ffmpeg_set_av_error(char* err, size_t err_len, const char* prefix, int code) {
	char detail[AV_ERROR_MAX_STRING_SIZE] = {0};
	char message[256] = {0};
	av_strerror(code, detail, sizeof(detail));
	if (prefix && prefix[0] != '\0') {
		snprintf(message, sizeof(message), "%s: %s", prefix, detail);
		io_ffmpeg_set_error(err, err_len, message);
		return;
	}
	io_ffmpeg_set_error(err, err_len, detail);
}

typedef struct io_ffmpeg_memory_reader {
	const uint8_t* data;
	size_t size;
	size_t pos;
} io_ffmpeg_memory_reader;

static int io_ffmpeg_read_packet(void* opaque, uint8_t* buf, int buf_size) {
	io_ffmpeg_memory_reader* reader = (io_ffmpeg_memory_reader*)opaque;
	size_t remaining;
	if (!reader || !buf || buf_size <= 0) {
		return AVERROR(EINVAL);
	}
	remaining = reader->size - reader->pos;
	if (remaining == 0) {
		return AVERROR_EOF;
	}
	if ((size_t)buf_size > remaining) {
		buf_size = (int)remaining;
	}
	memcpy(buf, reader->data + reader->pos, (size_t)buf_size);
	reader->pos += (size_t)buf_size;
	return buf_size;
}

static int64_t io_ffmpeg_seek(void* opaque, int64_t offset, int whence) {
	io_ffmpeg_memory_reader* reader = (io_ffmpeg_memory_reader*)opaque;
	int64_t next_pos;
	if (!reader) {
		return AVERROR(EINVAL);
	}
	if (whence == AVSEEK_SIZE) {
		return (int64_t)reader->size;
	}
	switch (whence) {
	case SEEK_SET:
		next_pos = offset;
		break;
	case SEEK_CUR:
		next_pos = (int64_t)reader->pos + offset;
		break;
	case SEEK_END:
		next_pos = (int64_t)reader->size + offset;
		break;
	default:
		return AVERROR(EINVAL);
	}
	if (next_pos < 0 || next_pos > (int64_t)reader->size) {
		return AVERROR(EINVAL);
	}
	reader->pos = (size_t)next_pos;
	return next_pos;
}

static enum AVPixelFormat io_ffmpeg_source_format(int samples, int bits) {
	if (bits == 8) {
		switch (samples) {
		case 1:
			return AV_PIX_FMT_GRAY8;
		case 3:
			return AV_PIX_FMT_RGB24;
		}
	}
	if (bits == 16) {
		switch (samples) {
		case 1:
			return AV_PIX_FMT_GRAY16LE;
		case 3:
			return AV_PIX_FMT_RGB48LE;
		}
	}
	return AV_PIX_FMT_NONE;
}

static enum AVPixelFormat io_ffmpeg_output_format(size_t dst_size, int width, int height) {
	size_t pixels;
	if (width <= 0 || height <= 0) {
		return AV_PIX_FMT_NONE;
	}
	pixels = (size_t)width * (size_t)height;
	if (dst_size == pixels) {
		return AV_PIX_FMT_GRAY8;
	}
	if (dst_size == pixels * 3u) {
		return AV_PIX_FMT_RGB24;
	}
	if (dst_size == pixels * 2u) {
		return AV_PIX_FMT_GRAY16LE;
	}
	if (dst_size == pixels * 6u) {
		return AV_PIX_FMT_RGB48LE;
	}
	return AV_PIX_FMT_NONE;
}

#if LIBAVCODEC_VERSION_MAJOR >= 61
static const enum AVPixelFormat* io_ffmpeg_supported_pixel_formats(const AVCodec* codec, int* count) {
	const void* configs = NULL;
	int num_configs = 0;

	if (count) {
		*count = 0;
	}
	if (!codec) {
		return NULL;
	}
	if (avcodec_get_supported_config(NULL, codec, AV_CODEC_CONFIG_PIX_FORMAT, 0, &configs, &num_configs) < 0) {
		return NULL;
	}
	if (count) {
		*count = num_configs;
	}
	return (const enum AVPixelFormat*)configs;
}
#else
static const enum AVPixelFormat* io_ffmpeg_supported_pixel_formats(const AVCodec* codec, int* count) {
	const enum AVPixelFormat* current;
	int num_configs = 0;

	if (count) {
		*count = 0;
	}
	if (!codec || !codec->pix_fmts) {
		return NULL;
	}
	for (current = codec->pix_fmts; *current != AV_PIX_FMT_NONE; ++current) {
		++num_configs;
	}
	if (count) {
		*count = num_configs;
	}
	return codec->pix_fmts;
}
#endif

static enum AVPixelFormat io_ffmpeg_choose_encoder_format(const AVCodec* codec, enum AVPixelFormat src_fmt) {
	const enum AVPixelFormat* supported = NULL;
	const enum AVPixelFormat* current;
	int count = 0;

	supported = io_ffmpeg_supported_pixel_formats(codec, &count);
	if (!supported || count <= 0) {
		return src_fmt;
	}
	for (current = supported; *current != AV_PIX_FMT_NONE; ++current) {
		if (*current == src_fmt) {
			return *current;
		}
	}
	if (src_fmt == AV_PIX_FMT_GRAY8) {
		for (current = supported; *current != AV_PIX_FMT_NONE; ++current) {
			if (*current == AV_PIX_FMT_YUV420P || *current == AV_PIX_FMT_GRAY8) {
				return *current;
			}
		}
	}
	if (src_fmt == AV_PIX_FMT_RGB24) {
		for (current = supported; *current != AV_PIX_FMT_NONE; ++current) {
			if (*current == AV_PIX_FMT_YUV420P || *current == AV_PIX_FMT_YUV444P || *current == AV_PIX_FMT_RGB24) {
				return *current;
			}
		}
	}
	if (src_fmt == AV_PIX_FMT_GRAY16LE || src_fmt == AV_PIX_FMT_RGB48LE) {
		for (current = supported; *current != AV_PIX_FMT_NONE; ++current) {
			if (*current == src_fmt || *current == AV_PIX_FMT_GRAY16LE || *current == AV_PIX_FMT_RGB48LE || *current == AV_PIX_FMT_YUV444P16LE) {
				return *current;
			}
		}
	}
	return supported[0];
}

static int io_ffmpeg_copy_or_scale_frame(const uint8_t* src, size_t src_size,
		int width, int height, enum AVPixelFormat src_fmt,
		AVFrame* frame, char* err, size_t err_len) {
	uint8_t* src_data[4] = {0};
	int src_linesize[4] = {0};
	int ret;
	struct SwsContext* sws = NULL;

	ret = av_image_fill_arrays(src_data, src_linesize, src, src_fmt, width, height, 1);
	if (ret < 0) {
		io_ffmpeg_set_av_error(err, err_len, "av_image_fill_arrays failed", ret);
		return -1;
	}
	if ((size_t)ret > src_size) {
		io_ffmpeg_set_error(err, err_len, "raw payload size does not match image dimensions");
		return -1;
	}

	ret = av_frame_make_writable(frame);
	if (ret < 0) {
		io_ffmpeg_set_av_error(err, err_len, "av_frame_make_writable failed", ret);
		return -1;
	}
	if (frame->format == src_fmt) {
		av_image_copy(frame->data, frame->linesize,
			(const uint8_t* const*)src_data, src_linesize,
			src_fmt, width, height);
		return 0;
	}

	sws = sws_getContext(width, height, src_fmt, width, height,
		(enum AVPixelFormat)frame->format, SWS_BILINEAR, NULL, NULL, NULL);
	if (!sws) {
		io_ffmpeg_set_error(err, err_len, "unsupported raw pixel layout");
		return -1;
	}
	sws_scale(sws, (const uint8_t* const*)src_data, src_linesize, 0, height, frame->data, frame->linesize);
	sws_freeContext(sws);
	return 0;
}

int io_ffmpeg_encode(const uint8_t* src, size_t src_size,
		int width, int height, int samples, int bits,
		int codec_id, const char* format_name,
		uint8_t** out_data, size_t* out_size, char* err, size_t err_len) {
	const AVCodec* codec = NULL;
	AVCodecContext* codec_ctx = NULL;
	AVFormatContext* format_ctx = NULL;
	AVStream* stream = NULL;
	AVFrame* frame = NULL;
	AVPacket* packet = NULL;
	uint8_t* dynamic_buf = NULL;
	uint8_t* output_copy = NULL;
	int output_size = 0;
	int ret = 0;
	enum AVPixelFormat src_fmt;
	enum AVPixelFormat enc_fmt;

	if (!src || !out_data || !out_size || !format_name || width <= 0 || height <= 0) {
		io_ffmpeg_set_error(err, err_len, "invalid ffmpeg encode parameters");
		return -1;
	}
	src_fmt = io_ffmpeg_source_format(samples, bits);
	if (src_fmt == AV_PIX_FMT_NONE) {
		io_ffmpeg_set_error(err, err_len, "unsupported raw pixel layout");
		return -1;
	}
	if ((size_t)av_image_get_buffer_size(src_fmt, width, height, 1) != src_size) {
		io_ffmpeg_set_error(err, err_len, "raw payload size does not match image dimensions");
		return -1;
	}

	codec = avcodec_find_encoder((enum AVCodecID)codec_id);
	if (!codec) {
		io_ffmpeg_set_error(err, err_len, "requested encoder is unavailable");
		return -1;
	}

	ret = avformat_alloc_output_context2(&format_ctx, NULL, format_name, NULL);
	if (ret < 0 || !format_ctx) {
		io_ffmpeg_set_av_error(err, err_len, "avformat_alloc_output_context2 failed", ret < 0 ? ret : AVERROR(EINVAL));
		return -1;
	}

	stream = avformat_new_stream(format_ctx, NULL);
	if (!stream) {
		io_ffmpeg_set_error(err, err_len, "avformat_new_stream failed");
		ret = AVERROR(ENOMEM);
		goto cleanup;
	}

	codec_ctx = avcodec_alloc_context3(codec);
	if (!codec_ctx) {
		io_ffmpeg_set_error(err, err_len, "avcodec_alloc_context3 failed");
		ret = AVERROR(ENOMEM);
		goto cleanup;
	}

	enc_fmt = io_ffmpeg_choose_encoder_format(codec, src_fmt);
	if (enc_fmt == AV_PIX_FMT_NONE) {
		io_ffmpeg_set_error(err, err_len, "unsupported raw pixel layout");
		ret = AVERROR(EINVAL);
		goto cleanup;
	}

	codec_ctx->codec_type = AVMEDIA_TYPE_VIDEO;
	codec_ctx->codec_id = (enum AVCodecID)codec_id;
	codec_ctx->width = width;
	codec_ctx->height = height;
	codec_ctx->pix_fmt = enc_fmt;
	codec_ctx->time_base = (AVRational){1, 1};
	codec_ctx->framerate = (AVRational){1, 1};
	codec_ctx->gop_size = 1;
	codec_ctx->max_b_frames = 0;
	codec_ctx->color_range = AVCOL_RANGE_JPEG;
	if (format_ctx->oformat->flags & AVFMT_GLOBALHEADER) {
		codec_ctx->flags |= AV_CODEC_FLAG_GLOBAL_HEADER;
	}

	ret = avcodec_open2(codec_ctx, codec, NULL);
	if (ret < 0) {
		io_ffmpeg_set_av_error(err, err_len, "avcodec_open2 failed", ret);
		goto cleanup;
	}

	ret = avcodec_parameters_from_context(stream->codecpar, codec_ctx);
	if (ret < 0) {
		io_ffmpeg_set_av_error(err, err_len, "avcodec_parameters_from_context failed", ret);
		goto cleanup;
	}
	stream->time_base = codec_ctx->time_base;

	ret = avio_open_dyn_buf(&format_ctx->pb);
	if (ret < 0) {
		io_ffmpeg_set_av_error(err, err_len, "avio_open_dyn_buf failed", ret);
		goto cleanup;
	}

	ret = avformat_write_header(format_ctx, NULL);
	if (ret < 0) {
		io_ffmpeg_set_av_error(err, err_len, "avformat_write_header failed", ret);
		goto cleanup;
	}

	frame = av_frame_alloc();
	packet = av_packet_alloc();
	if (!frame || !packet) {
		io_ffmpeg_set_error(err, err_len, "failed to allocate ffmpeg frame state");
		ret = AVERROR(ENOMEM);
		goto cleanup;
	}
	frame->format = codec_ctx->pix_fmt;
	frame->width = codec_ctx->width;
	frame->height = codec_ctx->height;
	frame->pts = 0;

	ret = av_frame_get_buffer(frame, 32);
	if (ret < 0) {
		io_ffmpeg_set_av_error(err, err_len, "av_frame_get_buffer failed", ret);
		goto cleanup;
	}
	if (io_ffmpeg_copy_or_scale_frame(src, src_size, width, height, src_fmt, frame, err, err_len) != 0) {
		ret = AVERROR(EINVAL);
		goto cleanup;
	}

	ret = avcodec_send_frame(codec_ctx, frame);
	if (ret < 0) {
		io_ffmpeg_set_av_error(err, err_len, "avcodec_send_frame failed", ret);
		goto cleanup;
	}
	for (;;) {
		ret = avcodec_receive_packet(codec_ctx, packet);
		if (ret == AVERROR(EAGAIN) || ret == AVERROR_EOF) {
			break;
		}
		if (ret < 0) {
			io_ffmpeg_set_av_error(err, err_len, "avcodec_receive_packet failed", ret);
			goto cleanup;
		}
		av_packet_rescale_ts(packet, codec_ctx->time_base, stream->time_base);
		packet->stream_index = stream->index;
		ret = av_interleaved_write_frame(format_ctx, packet);
		av_packet_unref(packet);
		if (ret < 0) {
			io_ffmpeg_set_av_error(err, err_len, "av_interleaved_write_frame failed", ret);
			goto cleanup;
		}
	}

	ret = avcodec_send_frame(codec_ctx, NULL);
	if (ret < 0 && ret != AVERROR_EOF) {
		io_ffmpeg_set_av_error(err, err_len, "avcodec_send_frame flush failed", ret);
		goto cleanup;
	}
	for (;;) {
		ret = avcodec_receive_packet(codec_ctx, packet);
		if (ret == AVERROR(EAGAIN) || ret == AVERROR_EOF) {
			break;
		}
		if (ret < 0) {
			io_ffmpeg_set_av_error(err, err_len, "avcodec_receive_packet flush failed", ret);
			goto cleanup;
		}
		av_packet_rescale_ts(packet, codec_ctx->time_base, stream->time_base);
		packet->stream_index = stream->index;
		ret = av_interleaved_write_frame(format_ctx, packet);
		av_packet_unref(packet);
		if (ret < 0) {
			io_ffmpeg_set_av_error(err, err_len, "av_interleaved_write_frame flush failed", ret);
			goto cleanup;
		}
	}

	ret = av_write_trailer(format_ctx);
	if (ret < 0) {
		io_ffmpeg_set_av_error(err, err_len, "av_write_trailer failed", ret);
		goto cleanup;
	}

	output_size = avio_close_dyn_buf(format_ctx->pb, &dynamic_buf);
	format_ctx->pb = NULL;
	if (output_size <= 0 || !dynamic_buf) {
		io_ffmpeg_set_error(err, err_len, "ffmpeg encode produced empty payload");
		ret = AVERROR(EINVAL);
		goto cleanup;
	}
	output_copy = (uint8_t*)malloc((size_t)output_size);
	if (!output_copy) {
		io_ffmpeg_set_error(err, err_len, "failed to allocate ffmpeg output buffer");
		ret = AVERROR(ENOMEM);
		goto cleanup;
	}
	memcpy(output_copy, dynamic_buf, (size_t)output_size);
	av_free(dynamic_buf);
	dynamic_buf = NULL;
	*out_data = output_copy;
	*out_size = (size_t)output_size;

	ret = 0;

cleanup:
	if (ret != 0 && format_ctx && format_ctx->pb) {
		avio_close_dyn_buf(format_ctx->pb, &dynamic_buf);
		format_ctx->pb = NULL;
	}
	if (dynamic_buf) {
		av_free(dynamic_buf);
	}
	if (packet) {
		av_packet_free(&packet);
	}
	if (frame) {
		av_frame_free(&frame);
	}
	if (codec_ctx) {
		avcodec_free_context(&codec_ctx);
	}
	if (format_ctx) {
		avformat_free_context(format_ctx);
	}
	if (ret != 0 && output_copy) {
		free(output_copy);
	}
	return ret == 0 ? 0 : -1;
}

int io_ffmpeg_decode(const uint8_t* src, size_t src_size,
		uint8_t* dst, size_t dst_size, char* err, size_t err_len) {
	AVFormatContext* format_ctx = NULL;
	AVCodecContext* codec_ctx = NULL;
	const AVCodec* codec = NULL;
	AVPacket* packet = NULL;
	AVFrame* frame = NULL;
	AVIOContext* avio = NULL;
	uint8_t* avio_buffer = NULL;
	struct SwsContext* sws = NULL;
	io_ffmpeg_memory_reader reader;
	int stream_index;
	int ret = 0;
	enum AVPixelFormat dst_fmt;
	uint8_t* dst_data[4] = {0};
	int dst_linesize[4] = {0};

	if (!src || src_size == 0 || !dst || dst_size == 0) {
		io_ffmpeg_set_error(err, err_len, "invalid ffmpeg decode parameters");
		return -1;
	}

	memset(&reader, 0, sizeof(reader));
	reader.data = src;
	reader.size = src_size;

	format_ctx = avformat_alloc_context();
	if (!format_ctx) {
		io_ffmpeg_set_error(err, err_len, "avformat_alloc_context failed");
		return -1;
	}

	avio_buffer = av_malloc(4096);
	if (!avio_buffer) {
		io_ffmpeg_set_error(err, err_len, "failed to allocate ffmpeg IO buffer");
		ret = AVERROR(ENOMEM);
		goto cleanup;
	}
	avio = avio_alloc_context(avio_buffer, 4096, 0, &reader, io_ffmpeg_read_packet, NULL, io_ffmpeg_seek);
	if (!avio) {
		io_ffmpeg_set_error(err, err_len, "avio_alloc_context failed");
		ret = AVERROR(ENOMEM);
		goto cleanup;
	}
	format_ctx->pb = avio;
	format_ctx->flags |= AVFMT_FLAG_CUSTOM_IO;

	ret = avformat_open_input(&format_ctx, NULL, NULL, NULL);
	if (ret < 0) {
		io_ffmpeg_set_av_error(err, err_len, "avformat_open_input failed", ret);
		goto cleanup;
	}
	ret = avformat_find_stream_info(format_ctx, NULL);
	if (ret < 0) {
		io_ffmpeg_set_av_error(err, err_len, "avformat_find_stream_info failed", ret);
		goto cleanup;
	}

	stream_index = av_find_best_stream(format_ctx, AVMEDIA_TYPE_VIDEO, -1, -1, &codec, 0);
	if (stream_index < 0 || !codec) {
		io_ffmpeg_set_error(err, err_len, "no video stream found");
		ret = AVERROR_STREAM_NOT_FOUND;
		goto cleanup;
	}

	codec_ctx = avcodec_alloc_context3(codec);
	if (!codec_ctx) {
		io_ffmpeg_set_error(err, err_len, "avcodec_alloc_context3 failed");
		ret = AVERROR(ENOMEM);
		goto cleanup;
	}
	ret = avcodec_parameters_to_context(codec_ctx, format_ctx->streams[stream_index]->codecpar);
	if (ret < 0) {
		io_ffmpeg_set_av_error(err, err_len, "avcodec_parameters_to_context failed", ret);
		goto cleanup;
	}
	ret = avcodec_open2(codec_ctx, codec, NULL);
	if (ret < 0) {
		io_ffmpeg_set_av_error(err, err_len, "avcodec_open2 failed", ret);
		goto cleanup;
	}

	packet = av_packet_alloc();
	frame = av_frame_alloc();
	if (!packet || !frame) {
		io_ffmpeg_set_error(err, err_len, "failed to allocate ffmpeg decode state");
		ret = AVERROR(ENOMEM);
		goto cleanup;
	}

	for (;;) {
		ret = av_read_frame(format_ctx, packet);
		if (ret == AVERROR_EOF) {
			ret = avcodec_send_packet(codec_ctx, NULL);
			if (ret < 0 && ret != AVERROR_EOF) {
				io_ffmpeg_set_av_error(err, err_len, "avcodec_send_packet flush failed", ret);
				goto cleanup;
			}
		} else if (ret < 0) {
			io_ffmpeg_set_av_error(err, err_len, "av_read_frame failed", ret);
			goto cleanup;
		} else if (packet->stream_index == stream_index) {
			ret = avcodec_send_packet(codec_ctx, packet);
			av_packet_unref(packet);
			if (ret < 0) {
				io_ffmpeg_set_av_error(err, err_len, "avcodec_send_packet failed", ret);
				goto cleanup;
			}
		} else {
			av_packet_unref(packet);
			continue;
		}

		for (;;) {
			ret = avcodec_receive_frame(codec_ctx, frame);
			if (ret == AVERROR(EAGAIN)) {
				break;
			}
			if (ret == AVERROR_EOF) {
				io_ffmpeg_set_error(err, err_len, "no decodable video frame found");
				goto cleanup;
			}
			if (ret < 0) {
				io_ffmpeg_set_av_error(err, err_len, "avcodec_receive_frame failed", ret);
				goto cleanup;
			}

			dst_fmt = io_ffmpeg_output_format(dst_size, frame->width, frame->height);
			if (dst_fmt == AV_PIX_FMT_NONE) {
				io_ffmpeg_set_error(err, err_len, "decoded payload does not match frame dimensions");
				ret = AVERROR(EINVAL);
				goto cleanup;
			}
			ret = av_image_fill_arrays(dst_data, dst_linesize, dst, dst_fmt, frame->width, frame->height, 1);
			if (ret < 0) {
				io_ffmpeg_set_av_error(err, err_len, "av_image_fill_arrays failed", ret);
				goto cleanup;
			}
			sws = sws_getContext(frame->width, frame->height, (enum AVPixelFormat)frame->format,
				frame->width, frame->height, dst_fmt, SWS_BILINEAR, NULL, NULL, NULL);
			if (!sws) {
				io_ffmpeg_set_error(err, err_len, "unsupported decoded frame layout");
				ret = AVERROR(EINVAL);
				goto cleanup;
			}
			sws_scale(sws, (const uint8_t* const*)frame->data, frame->linesize, 0, frame->height, dst_data, dst_linesize);
			ret = 0;
			goto cleanup;
		}
	}

cleanup:
	if (sws) {
		sws_freeContext(sws);
	}
	if (packet) {
		av_packet_free(&packet);
	}
	if (frame) {
		av_frame_free(&frame);
	}
	if (codec_ctx) {
		avcodec_free_context(&codec_ctx);
	}
	if (format_ctx) {
		avformat_close_input(&format_ctx);
	}
	if (avio) {
		av_freep(&avio->buffer);
		avio_context_free(&avio);
	}
	return ret == 0 ? 0 : -1;
}