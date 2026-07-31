package llvm

type funcSig struct {
	ret    string
	params []string
}

var funcSigOverrides = map[string]funcSig{}

// SetFuncSig registers a textual LLVM signature override.
// Use empty strings to keep defaults for ret or any param.
func SetFuncSig(name, ret string, params []string) {
	cp := make([]string, len(params))
	copy(cp, params)
	funcSigOverrides[name] = funcSig{ret: ret, params: cp}
}

func getFuncSig(name string) (funcSig, bool) {
	s, ok := funcSigOverrides[name]
	return s, ok
}

func init() {
	// Dict runtime overrides
	SetFuncSig("dict_new", "ptr", nil)
	SetFuncSig("dict_insert", "void", nil)
	SetFuncSig("dict_insert_val", "void", nil)
	SetFuncSig("dict_get", "ptr", nil)
	SetFuncSig("dict_keys", "ptr", nil)
	SetFuncSig("dict_values", "ptr", nil)
	SetFuncSig("dict_clear", "void", nil)
	SetFuncSig("dict_values", "ptr", nil)
	SetFuncSig("dict_clear", "void", nil)
	SetFuncSig("dict_free", "void", nil)

	// Set runtime overrides
	SetFuncSig("set_new", "ptr", nil)
	SetFuncSig("set_add", "void", nil)
	SetFuncSig("set_remove", "void", nil)
	SetFuncSig("set_contains", "i1", nil)
	SetFuncSig("set_clear", "void", nil)
	SetFuncSig("set_free", "void", nil)
	SetFuncSig("set_to_array", "ptr", nil)
	SetFuncSig("set_union", "ptr", nil)
	SetFuncSig("set_intersection", "ptr", nil)
	SetFuncSig("set_difference", "ptr", nil)
	SetFuncSig("bool_to_cstring", "ptr", nil)
	SetFuncSig("__desi_str_new", "ptr", nil)
	SetFuncSig("__desi_str_append_free", "ptr", nil)

	// Range runtime overrides. Ranges work in i64 throughout; the lowerer
	// narrows to i32 when it binds the loop variable.
	SetFuncSig("range_new", "ptr", []string{"i64", "i64", "i64"})
	SetFuncSig("range_len", "i64", []string{"ptr"})
	SetFuncSig("range_get", "i64", []string{"ptr", "i64"})
	SetFuncSig("range_contains", "i1", []string{"ptr", "i64"})
	SetFuncSig("range_free", "void", []string{"ptr"})

	// List runtime overrides
	SetFuncSig("list_new", "ptr", nil)
	SetFuncSig("list_append", "void", nil)
	SetFuncSig("list_get", "ptr", nil)
	SetFuncSig("list_set", "void", nil)
	SetFuncSig("list_len", "i64", nil)
	SetFuncSig("list_slice", "ptr", nil)
	SetFuncSig("list_free", "void", nil)
	SetFuncSig("list_copy", "ptr", nil)
	SetFuncSig("list_extend", "void", nil)
	SetFuncSig("list_insert", "void", nil)
	SetFuncSig("list_pop", "ptr", nil)
	SetFuncSig("list_remove", "void", nil)
	SetFuncSig("list_reverse", "void", nil)
	SetFuncSig("list_index", "i64", nil)
	SetFuncSig("list_count", "i64", nil)
	SetFuncSig("list_contains", "i1", nil)
	SetFuncSig("list_map", "ptr", nil)
	SetFuncSig("list_filter", "ptr", nil)
	SetFuncSig("list_reduce", "ptr", nil)
	SetFuncSig("list_any", "i1", nil)
	SetFuncSig("list_all", "i1", nil)

	// Async/future runtime overrides
	SetFuncSig("__future_new", "ptr", nil)
	SetFuncSig("__future_complete", "void", []string{"ptr", "i64"})
	SetFuncSig("__await_blocking", "i64", []string{"ptr"})
	SetFuncSig("__future_spawn_0", "void", []string{"ptr", "ptr"})
	SetFuncSig("__future_spawn_1", "void", []string{"ptr", "ptr", "i64"})
	SetFuncSig("__future_spawn_2", "void", []string{"ptr", "ptr", "i64", "i64"})
	SetFuncSig("__future_spawn_3", "void", []string{"ptr", "ptr", "i64", "i64", "i64"})
	SetFuncSig("__future_spawn_4", "void", []string{"ptr", "ptr", "i64", "i64", "i64", "i64"})

	// HTTP server runtime overrides
	SetFuncSig("__http_server_new", "ptr", nil)
	SetFuncSig("__http_server_new_tls", "ptr", nil)
	SetFuncSig("__http_req_method", "ptr", nil)
	SetFuncSig("__http_req_path", "ptr", nil)
	SetFuncSig("__http_req_body", "ptr", nil)
	SetFuncSig("__http_req_header", "ptr", nil)
	SetFuncSig("__http_req_query", "ptr", nil)
	SetFuncSig("__http_req_param", "ptr", nil)
	SetFuncSig("__http_req_path_param", "ptr", nil)
	SetFuncSig("__http_req_json", "ptr", nil)
	SetFuncSig("__http_resp_new", "ptr", nil)
	SetFuncSig("__http_resp_from_file", "ptr", nil)
	SetFuncSig("__http_request_no_redirect", "ptr", nil)
	SetFuncSig("__http_server_set_handler", "void", nil)
	SetFuncSig("__http_server_route", "void", nil)
	SetFuncSig("__http_server_run", "void", nil)
	SetFuncSig("__http_server_static", "void", nil)
	SetFuncSig("__http_server_max_body", "void", nil)
	SetFuncSig("__http_server_timeout", "void", nil)
	SetFuncSig("__http_server_use", "void", nil)
	SetFuncSig("__http_server_rate_limit", "void", nil)
	SetFuncSig("__http_resp_header", "void", nil)
	SetFuncSig("__http_server_cors", "void", nil)
	SetFuncSig("__http_req_cookie", "ptr", nil)
	SetFuncSig("__http_resp_cookie", "void", nil)

	// Shutdown
	SetFuncSig("__http_server_shutdown", "void", nil)

	// Multipart form accessors
	SetFuncSig("__http_req_form_field", "ptr", nil)
	SetFuncSig("__http_req_form_file", "ptr", nil)
	SetFuncSig("__http_req_form_filename", "ptr", nil)
	SetFuncSig("__http_req_form_file_size", "i32", nil)

	// SSE (Server-Sent Events)
	SetFuncSig("__http_sse_start", "i32", nil)
	SetFuncSig("__http_sse_send_data", "void", nil)
	SetFuncSig("__http_sse_send", "void", nil)
	SetFuncSig("__http_sse_close", "void", nil)
	SetFuncSig("__http_sse_response", "ptr", nil)

	// Client cookie jar
	SetFuncSig("__http_enable_cookies", "void", nil)
	SetFuncSig("__http_disable_cookies", "void", nil)
	SetFuncSig("__http_clear_cookies", "void", nil)

	// TLS certificate configuration
	SetFuncSig("__http_set_ca_bundle", "void", nil)
	SetFuncSig("__http_set_client_cert", "void", nil)

	// Proxy configuration
	SetFuncSig("__http_set_proxy", "void", nil)
	SetFuncSig("__http_clear_proxy", "void", nil)

	// WebSocket runtime overrides
	SetFuncSig("__ws_send", "void", nil)
	SetFuncSig("__ws_send_binary", "void", nil)
	SetFuncSig("__ws_broadcast", "void", nil)
	SetFuncSig("__ws_close", "void", nil)
	SetFuncSig("__ws_join", "void", nil)
	SetFuncSig("__ws_leave", "void", nil)
	SetFuncSig("__ws_to_room", "void", nil)
	SetFuncSig("__ws_set_path", "void", nil)
	SetFuncSig("__ws_set_on_message", "void", nil)
	SetFuncSig("__ws_set_on_open", "void", nil)
	SetFuncSig("__ws_set_on_close", "void", nil)
	SetFuncSig("__ws_set_max_message_size", "void", nil)
	SetFuncSig("__ws_set_ping_interval", "void", nil)
	SetFuncSig("__ws_set_compression", "void", nil)
	SetFuncSig("__ws_route", "void", nil)
	SetFuncSig("__ws_route_on_open", "void", nil)
	SetFuncSig("__ws_route_on_close", "void", nil)
	SetFuncSig("__ws_route_on_binary", "void", nil)
	SetFuncSig("__ws_conn_count", "i32", nil)

	// JSON runtime overrides
	SetFuncSig("__json_parse", "ptr", nil)
	SetFuncSig("__json_stringify", "ptr", nil)
	SetFuncSig("__json_get_string", "ptr", nil)
	SetFuncSig("__json_array_get", "ptr", nil)
	SetFuncSig("__json_object_get", "ptr", nil)
	SetFuncSig("__json_object_key", "ptr", nil)
	SetFuncSig("__json_new_object", "ptr", nil)
	SetFuncSig("__json_new_array", "ptr", nil)
	SetFuncSig("__json_new_string", "ptr", nil)
	SetFuncSig("__json_new_number", "ptr", nil)
	SetFuncSig("__json_new_bool", "ptr", nil)
	SetFuncSig("__json_new_null", "ptr", nil)
	SetFuncSig("__json_object_set", "void", nil)
	SetFuncSig("__json_array_push", "void", nil)
	SetFuncSig("__json_object_remove", "void", nil)
	SetFuncSig("__json_object_keys", "ptr", nil)
	SetFuncSig("__json_free", "void", nil)

	// Bytes runtime overrides
	SetFuncSig("__bytes_new", "ptr", nil)
	SetFuncSig("__bytes_from_str", "ptr", nil)
	SetFuncSig("__bytes_to_str", "ptr", nil)
	SetFuncSig("__bytes_len", "i32", nil)
	SetFuncSig("__bytes_get", "i32", nil)
	SetFuncSig("__bytes_set", "void", nil)
	SetFuncSig("__bytes_slice", "ptr", nil)
	SetFuncSig("__bytes_concat", "ptr", nil)
	SetFuncSig("__bytes_equal", "i32", nil)
	SetFuncSig("__bytes_free", "void", nil)
	SetFuncSig("__bytes_to_hex", "ptr", nil)
	SetFuncSig("__bytes_from_hex", "ptr", nil)
	SetFuncSig("__bytes_repeat", "ptr", nil)
	SetFuncSig("__bytes_index_of", "i32", nil)
}
