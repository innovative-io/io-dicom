//go:build openjph && cgo
// +build openjph,cgo

#include <openjph/ojph_base.h>
#include <openjph/ojph_codestream.h>
#include <openjph/ojph_file.h>
#include <openjph/ojph_mem.h>
#include <openjph/ojph_params.h>

#include <algorithm>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <exception>

namespace {

void io_openjph_set_error(char* err, size_t err_len, const char* msg) {
	if (!err || err_len == 0) {
		return;
	}
	if (!msg) {
		err[0] = '\0';
		return;
	}
	std::snprintf(err, err_len, "%s", msg);
}

int io_openjph_num_decompositions(uint16_t width, uint16_t height) {
	int min_dim = std::min<int>(width, height);
	int decompositions = 0;
	while (min_dim > 1 && decompositions < 5) {
		min_dim >>= 1;
		decompositions++;
	}
	return decompositions;
}

bool io_openjph_copy_raw_to_line(const uint8_t* src, size_t src_size,
		ojph::line_buf* line, uint32_t width, uint16_t samples,
		uint16_t bitsa, uint16_t comp, uint32_t row) {
	if (!src || !line || !(samples == 1 || samples == 3) || !(bitsa == 8 || bitsa == 12 || bitsa == 16)) {
		return false;
	}
	const size_t bytes_per_sample = bitsa > 8 ? 2u : 1u;
	const size_t row_stride = static_cast<size_t>(width) * static_cast<size_t>(samples) * bytes_per_sample;
	const size_t row_offset = static_cast<size_t>(row) * row_stride;
	if (row_offset + row_stride > src_size) {
		return false;
	}
	for (uint32_t x = 0; x < width; ++x) {
		const size_t sample_offset = row_offset + (static_cast<size_t>(x) * samples + comp) * bytes_per_sample;
		if (bytes_per_sample == 1) {
			line->i32[x] = src[sample_offset];
		} else {
			line->i32[x] = (static_cast<int32_t>(src[sample_offset]) << 8) | static_cast<int32_t>(src[sample_offset + 1]);
		}
	}
	return true;
}

bool io_openjph_copy_line_to_raw(const ojph::line_buf* line, uint8_t* dst, size_t dst_size,
		uint32_t width, uint16_t samples, uint16_t bitsa, uint16_t comp, uint32_t row, uint16_t max_sample) {
	if (!line || !dst || !(samples == 1 || samples == 3) || !(bitsa == 8 || bitsa == 12 || bitsa == 16)) {
		return false;
	}
	const size_t bytes_per_sample = bitsa > 8 ? 2u : 1u;
	const size_t row_stride = static_cast<size_t>(width) * static_cast<size_t>(samples) * bytes_per_sample;
	const size_t row_offset = static_cast<size_t>(row) * row_stride;
	if (row_offset + row_stride > dst_size) {
		return false;
	}
	for (uint32_t x = 0; x < width; ++x) {
		const int32_t sample = line->i32[x];
		if (sample < 0 || sample > max_sample) {
			return false;
		}
		const size_t sample_offset = row_offset + (static_cast<size_t>(x) * samples + comp) * bytes_per_sample;
		if (bytes_per_sample == 1) {
			dst[sample_offset] = static_cast<uint8_t>(sample);
		} else {
			dst[sample_offset] = static_cast<uint8_t>((sample >> 8) & 0xFF);
			dst[sample_offset + 1] = static_cast<uint8_t>(sample & 0xFF);
		}
	}
	return true;
}

}  // namespace

extern "C" int io_openjph_encode(const uint8_t* src, size_t src_size,
		uint16_t width, uint16_t height, uint16_t samples, uint16_t bitsa,
		uint8_t** out_data, size_t* out_size, char* err, size_t err_len) {
	if (!src || !out_data || !out_size || width == 0 || height == 0 || !(samples == 1 || samples == 3) || !(bitsa == 8 || bitsa == 12 || bitsa == 16)) {
		io_openjph_set_error(err, err_len, "unsupported OpenJPH image layout");
		return -1;
	}
	const size_t expected = static_cast<size_t>(width) * static_cast<size_t>(height) * static_cast<size_t>(samples) * (bitsa > 8 ? 2u : 1u);
	if (expected == 0 || expected != src_size) {
		io_openjph_set_error(err, err_len, "raw payload size does not match image dimensions");
		return -1;
	}

	try {
		ojph::codestream codestream;
		ojph::param_siz siz = codestream.access_siz();
		siz.set_image_extent(ojph::point(width, height));
		siz.set_num_components(samples);
		for (uint16_t comp = 0; comp < samples; ++comp) {
			siz.set_component(comp, ojph::point(1, 1), bitsa, false);
		}
		siz.set_image_offset(ojph::point(0, 0));
		siz.set_tile_size(ojph::size(width, height));
		siz.set_tile_offset(ojph::point(0, 0));

		ojph::param_cod cod = codestream.access_cod();
		cod.set_num_decomposition(io_openjph_num_decompositions(width, height));
		cod.set_color_transform(false);
		cod.set_reversible(true);
		codestream.set_planar(false);

		ojph::mem_outfile outfile;
		outfile.open(std::max<size_t>(65536u, src_size), false);
		codestream.write_headers(&outfile);

		ojph::ui32 next_comp = 0;
		ojph::line_buf* line = codestream.exchange(NULL, next_comp);
		for (uint32_t row = 0; row < height; ++row) {
			for (uint16_t comp = 0; comp < samples; ++comp) {
				if (!line || next_comp != comp || !io_openjph_copy_raw_to_line(src, src_size, line, width, samples, bitsa, comp, row)) {
					io_openjph_set_error(err, err_len, "failed to feed image rows into OpenJPH encoder");
					return -1;
				}
				line = codestream.exchange(line, next_comp);
			}
		}

		codestream.flush();
		codestream.close();

		const size_t encoded_size = outfile.get_used_size();
		if (encoded_size == 0) {
			io_openjph_set_error(err, err_len, "OpenJPH encoder produced empty payload");
			return -1;
		}
		uint8_t* encoded = static_cast<uint8_t*>(std::malloc(encoded_size));
		if (!encoded) {
			io_openjph_set_error(err, err_len, "failed to allocate OpenJPH output buffer");
			return -1;
		}
		std::memcpy(encoded, outfile.get_data(), encoded_size);
		*out_data = encoded;
		*out_size = encoded_size;
		return 0;
	} catch (const std::exception& exc) {
		io_openjph_set_error(err, err_len, exc.what());
		return -1;
	} catch (...) {
		io_openjph_set_error(err, err_len, "OpenJPH encoder raised an unknown exception");
		return -1;
	}
}

extern "C" int io_openjph_decode(const uint8_t* src, size_t src_size,
		uint8_t* dst, size_t dst_size, char* err, size_t err_len) {
	if (!src || !dst || src_size == 0 || dst_size == 0) {
		io_openjph_set_error(err, err_len, "invalid OpenJPH decode parameters");
		return -1;
	}

	try {
		ojph::codestream codestream;
		ojph::mem_infile infile;
		infile.open(src, src_size);
		codestream.read_headers(&infile);
		codestream.set_planar(false);

		ojph::param_siz siz = codestream.access_siz();
		const uint32_t num_comps = siz.get_num_components();
		if (!(num_comps == 1 || num_comps == 3)) {
			io_openjph_set_error(err, err_len, "unsupported decoded component count");
			return -1;
		}
		const uint32_t width = siz.get_recon_width(0);
		const uint32_t height = siz.get_recon_height(0);
		const uint16_t bitsa = static_cast<uint16_t>(siz.get_bit_depth(0));
		if (!(bitsa == 8 || bitsa == 12 || bitsa == 16)) {
			io_openjph_set_error(err, err_len, "unsupported decoded precision");
			return -1;
		}
		for (uint32_t comp = 0; comp < num_comps; ++comp) {
			if (siz.is_signed(comp) || siz.get_recon_width(comp) != width || siz.get_recon_height(comp) != height) {
				io_openjph_set_error(err, err_len, "unsupported decoded image layout");
				return -1;
			}
			const ojph::point downsampling = siz.get_downsampling(comp);
			if (downsampling.x != 1 || downsampling.y != 1 || siz.get_bit_depth(comp) != bitsa) {
				io_openjph_set_error(err, err_len, "unsupported decoded image layout");
				return -1;
			}
		}

		const size_t expected = static_cast<size_t>(width) * static_cast<size_t>(height) * static_cast<size_t>(num_comps) * (bitsa > 8 ? 2u : 1u);
		if (expected > dst_size) {
			io_openjph_set_error(err, err_len, "decoded payload does not fit output buffer");
			return -1;
		}

		codestream.create();
		const uint16_t max_sample = bitsa == 8 ? 255u : (bitsa == 12 ? 4095u : 65535u);
		for (uint32_t row = 0; row < height; ++row) {
			for (uint16_t comp = 0; comp < num_comps; ++comp) {
				ojph::ui32 comp_num = 0;
				ojph::line_buf* line = codestream.pull(comp_num);
				if (!line || comp_num != comp || !io_openjph_copy_line_to_raw(line, dst, dst_size, width, static_cast<uint16_t>(num_comps), bitsa, comp, row, max_sample)) {
					io_openjph_set_error(err, err_len, "failed to extract decoded image rows from OpenJPH");
					return -1;
				}
			}
		}
		codestream.close();
		return 0;
	} catch (const std::exception& exc) {
		io_openjph_set_error(err, err_len, exc.what());
		return -1;
	} catch (...) {
		io_openjph_set_error(err, err_len, "OpenJPH decoder raised an unknown exception");
		return -1;
	}
}