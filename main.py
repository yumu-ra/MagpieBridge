def add(a, b):
    return a + b

def read_photo(mem_view):
    """
    将 memoryview 转换为 bytes 并写入本地文件保存为图片
    :param mem_view: 内存视图对象，对应照片的二进制数据
    :raises TypeError: 传入参数非 memoryview 类型
    """
    if not isinstance(mem_view, memoryview):
        raise TypeError(f"Expected memoryview type, got {type(mem_view)}")
    
    photo_data = bytes(mem_view)
    with open("received_photo.jpg", "wb") as f:
        f.write(photo_data)

def _process_photo(mem_view):
    """
    读取 memoryview 中指定范围的字节数据
    :param mem_view: 内存视图对象
    :raises TypeError: 传入参数非 memoryview 类型
    """
    if not isinstance(mem_view, memoryview):
        raise TypeError(f"Expected memoryview type, got {type(mem_view)}")
    
    # 边界保护：避免内存视图长度不足导致越界
    head = mem_view[:10] if len(mem_view) >= 10 else mem_view[:]
    end_idx = min(300, len(mem_view))
    part = mem_view[100:end_idx] if len(mem_view) >= 100 else b""

def len_photo(mem_view):
    """
    获取 memoryview 总字节数和只读属性
    :param mem_view: 内存视图对象
    :return: (total_size: int, readonly: bool)
    :raises TypeError: 传入参数非 memoryview 类型
    """
    if not isinstance(mem_view, memoryview):
        raise TypeError(f"Expected memoryview type, got {type(mem_view)}")
    
    total_size = len(mem_view)
    readonly = mem_view.readonly
    return total_size, readonly

def write_photo(mem_view):
    """
    向 memoryview 中写入指定数据（需确保内存视图可写）
    :param mem_view: 内存视图对象
    :raises TypeError: 传入参数非 memoryview 类型
    :raises PermissionError: 内存视图为只读
    :raises IndexError: 写入范围越界
    """
    if not isinstance(mem_view, memoryview):
        raise TypeError(f"Expected memoryview type, got {type(mem_view)}")
    
    if mem_view.readonly:
        raise PermissionError("Memory view is read-only, cannot write")
    
    # 边界校验：确保有足够长度写入数据
    if len(mem_view) < 4:
        raise IndexError(f"Memory view length {len(mem_view)} < 4, cannot write first 4 bytes")
    
    # 覆盖前 4 字节
    mem_view[:5] = b"\x00\x00\x00\x00\x00"
    
    # 写入自定义数据
    new_data = b"hello from python"
    write_start = 10
    write_end = write_start + len(new_data)
    
    if write_end > len(mem_view):
        raise IndexError(f"Write range out of bounds: memory view length {len(mem_view)}, write end {write_end}")
    
    mem_view[write_start:write_end] = new_data