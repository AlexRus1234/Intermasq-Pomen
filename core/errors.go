// Pomen - plugin for Intermasq
// Copyright (C) 2026 AlexRus1234
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package core

import "errors"

// Контракт ошибок между core и api. Слою API важно различать
// "клиент дал плохой запрос / ссылается на несуществующий объект" (4xx)
// от "что-то сломалось внутри" (5xx). Раньше api/routes.go на любой ошибке
// Engine возвращал 500, из-за чего smoke получал 500 там, где semantically
// нужен 400 или 404 (например, "контейнер без port-XXXX" — это bad request,
// а "маршрут не найден" — not found).
//
// Оборачивание через %w + errors.Is позволяет сохранить человекочитаемое
// сообщение и при этом дать handler'у однозначный критерий для HTTP-статуса.

var (
	// ErrBadRequest — некорректный ввод клиента: отсутствует обязательное
	// поле, ссылка на неизвестную ноду, контейнер без нужного label.
	ErrBadRequest = errors.New("bad request")

	// ErrNotFound — объект по запрошенному ключу не существует.
	ErrNotFound = errors.New("not found")
)
