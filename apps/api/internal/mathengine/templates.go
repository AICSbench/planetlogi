package mathengine

import (
	"fmt"
	"math/rand"
)

type TemplateMeta struct {
	Code           string
	Grade          int
	SkillCode      string
	SkillName      LocaleText
	CurriculumCode string
	Difficulty     int
	HeuristicCode  string
	HeuristicName  LocaleText
	InputType      InputType
	Generate       func(rng *rand.Rand, locale string, seed int64) GeneratedProblem
}

var kidNames = []string{"Rina", "Budi", "Maya", "Lucas", "Siti", "Ethan", "Nadia", "Raka"}

func pickName(rng *rand.Rand) string {
	return kidNames[rng.Intn(len(kidNames))]
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

// Registry returns all 60 deterministic procedural templates spanning Grades 1-6.
func Registry() []TemplateMeta {
	base := []TemplateMeta{
		// ==================== GRADE 1 ====================
		{
			Code: "g1_add_within_20", Grade: 1, SkillCode: "num-add-within-20",
			SkillName: LocaleText{EN: "Addition within 20", ID: "Penjumlahan sampai 20"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 1, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				name := pickName(rng)
				a := 4 + rng.Intn(8)
				b := 2 + rng.Intn(9)
				ans := a + b
				stem := LocaleText{
					EN: fmt.Sprintf("%s collects %d blue stars and %d gold stars. How many stars does %s have altogether?", name, a, b, name),
					ID: fmt.Sprintf("%s mengumpulkan %d bintang biru dan %d bintang emas. Berapa jumlah bintang %s seluruhnya?", name, a, b, name),
				}.For(locale)
				diag := &DiagramSpec{Type: "number_line", Params: map[string]any{"start": a, "jump": b, "end": ans}}
				return buildSingleNumeric("g1_add_within_20", seed, 1, "num-add-within-20",
					LocaleText{EN: "Addition within 20", ID: "Penjumlahan sampai 20"}, "SINGAPORE_MOE", 1, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans),
					LocaleText{EN: "stars", ID: "bintang"}.For(locale),
					[]string{LocaleText{EN: "Count on from the larger number.", ID: "Hitung maju dari bilangan yang lebih besar."}.For(locale)},
					diag,
					[]SolutionStep{
						{Text: LocaleText{EN: fmt.Sprintf("Start at %d on the number line and jump forward %d steps.", a, b), ID: fmt.Sprintf("Mulai dari %d pada garis bilangan lalu lompat maju %d langkah.", a, b)}.For(locale), Diagram: diag},
						{Text: fmt.Sprintf("%d + %d = %d", a, b, ans)},
					})
			},
		},
		{
			Code: "g1_sub_within_20", Grade: 1, SkillCode: "num-sub-within-20",
			SkillName: LocaleText{EN: "Subtraction within 20", ID: "Pengurangan sampai 20"},
			CurriculumCode: "KURIKULUM_MERDEKA", Difficulty: 1, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				name := pickName(rng)
				total := 11 + rng.Intn(9)
				away := 2 + rng.Intn(8)
				ans := total - away
				stem := LocaleText{
					EN: fmt.Sprintf("%s had %d space stickers and gave %d stickers to a friend. How many stickers are left?", name, total, away),
					ID: fmt.Sprintf("%s memiliki %d stiker antariksa lalu memberikan %d stiker kepada temannya. Berapa stiker yang tersisa?", name, total, away),
				}.For(locale)
				return buildSingleNumeric("g1_sub_within_20", seed, 1, "num-sub-within-20",
					LocaleText{EN: "Subtraction within 20", ID: "Pengurangan sampai 20"}, "KURIKULUM_MERDEKA", 1, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), LocaleText{EN: "stickers", ID: "stiker"}.For(locale),
					[]string{LocaleText{EN: "Subtract the stickers given away from the total.", ID: "Kurangi jumlah awal dengan stiker yang diberikan."}.For(locale)},
					nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d − %d = %d", total, away, ans)}})
			},
		},
		{
			Code: "g1_place_value_tens_ones", Grade: 1, SkillCode: "num-place-value-100",
			SkillName: LocaleText{EN: "Place Value (Tens & Ones)", ID: "Nilai Tempat (Puluhan & Satuan)"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 2, InputType: InputMultiNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				tens := 2 + rng.Intn(8)
				ones := 1 + rng.Intn(9)
				num := tens*10 + ones
				stem := LocaleText{
					EN: fmt.Sprintf("Break down the number %d into tens and ones.", num),
					ID: fmt.Sprintf("Uraikan bilangan %d menjadi puluhan dan satuan.", num),
				}.For(locale)
				return GeneratedProblem{
					Question: RenderedQuestion{
						ID: fmt.Sprintf("PL-G1-PV-%d", seed), TemplateCode: "g1_place_value_tens_ones", Seed: seed,
						Grade: 1, SkillCode: "num-place-value-100",
						SkillName: LocaleText{EN: "Place Value (Tens & Ones)", ID: "Nilai Tempat (Puluhan & Satuan)"}.For(locale),
						CurriculumCode: "SINGAPORE_MOE", Difficulty: 2, InputType: InputMultiNumeric, Locale: locale,
						Stem: stem, TTSText: VerbalizeMath(stem, locale),
						Hints: []string{LocaleText{EN: "The first digit shows tens; the second digit shows ones.", ID: "Angka pertama adalah puluhan; angka kedua adalah satuan."}.For(locale)},
						Parts: []InputPart{
							{Key: "tens", Label: LocaleText{EN: "Tens", ID: "Puluhan"}.For(locale)},
							{Key: "ones", Label: LocaleText{EN: "Ones", ID: "Satuan"}.For(locale)},
						},
					},
					Solution: WorkedSolution{
						CorrectAnswers: map[string]string{"tens": fmt.Sprintf("%d", tens), "ones": fmt.Sprintf("%d", ones)},
						Steps: []SolutionStep{
							{Text: LocaleText{EN: fmt.Sprintf("%d = %d tens and %d ones (%d + %d).", num, tens, ones, tens*10, ones), ID: fmt.Sprintf("%d = %d puluhan dan %d satuan (%d + %d).", num, tens, ones, tens*10, ones)}.For(locale)},
						},
					},
					GradingStrategy: "exact_numeric",
				}
			},
		},
		{
			Code: "g1_pattern_hots", Grade: 1, SkillCode: "hots-look-for-patterns-g1",
			SkillName: LocaleText{EN: "Number Patterns", ID: "Pola Bilangan"},
			CurriculumCode: "HOTS_WHEEL", Difficulty: 2, HeuristicCode: "LOOK_FOR_PATTERNS",
			HeuristicName: LocaleText{EN: "Look for Patterns", ID: "Mencari Pola"}, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				start := 2 + rng.Intn(5)
				step := 2 + rng.Intn(3)
				a, b, c, d := start, start+step, start+2*step, start+3*step
				ans := start + 4*step
				stem := LocaleText{
					EN: fmt.Sprintf("Find the next number in the star pattern: %d, %d, %d, %d, ___", a, b, c, d),
					ID: fmt.Sprintf("Temukan bilangan berikutnya pada pola bintang: %d, %d, %d, %d, ___", a, b, c, d),
				}.For(locale)
				return buildSingleNumeric("g1_pattern_hots", seed, 1, "hots-look-for-patterns-g1",
					LocaleText{EN: "Number Patterns", ID: "Pola Bilangan"}, "HOTS_WHEEL", 2,
					"LOOK_FOR_PATTERNS", LocaleText{EN: "Look for Patterns", ID: "Mencari Pola"},
					locale, stem, fmt.Sprintf("%d", ans), "",
					[]string{LocaleText{EN: fmt.Sprintf("Notice how much is added each time (+%d).", step), ID: fmt.Sprintf("Perhatikan berapa yang ditambahkan setiap langkah (+%d).", step)}.For(locale)},
					nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d + %d = %d", d, step, ans)}})
			},
		},

		// ==================== GRADE 2 ====================
		{
			Code: "g2_add_within_1000", Grade: 2, SkillCode: "num-add-1000",
			SkillName: LocaleText{EN: "Addition within 1,000", ID: "Penjumlahan sampai 1.000"},
			CurriculumCode: "KURIKULUM_MERDEKA", Difficulty: 2, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				a := 125 + rng.Intn(400)
				b := 115 + rng.Intn(350)
				ans := a + b
				stem := LocaleText{
					EN: fmt.Sprintf("A space station received %d moon rocks on Monday and %d moon rocks on Tuesday. How many moon rocks did it receive in total?", a, b),
					ID: fmt.Sprintf("Sebuah stasiun antariksa menerima %d batu bulan pada hari Senin dan %d batu bulan pada hari Selasa. Berapa total batu bulan yang diterima?", a, b),
				}.For(locale)
				return buildSingleNumeric("g2_add_within_1000", seed, 2, "num-add-1000",
					LocaleText{EN: "Addition within 1,000", ID: "Penjumlahan sampai 1.000"}, "KURIKULUM_MERDEKA", 2, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), LocaleText{EN: "rocks", ID: "batu"}.For(locale),
					[]string{LocaleText{EN: "Use the Column 4-Operations tool to add ones, tens, then hundreds.", ID: "Gunakan alat Kalkulator Susun untuk menjumlahkan satuan, puluhan, lalu ratusan."}.For(locale)},
					nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d + %d = %d", a, b, ans)}})
			},
		},
		{
			Code: "g2_mul_tables_2_5_10", Grade: 2, SkillCode: "num-mul-2-3-4-5-10",
			SkillName: LocaleText{EN: "Multiplication Tables (2–5, 10)", ID: "Perkalian 2–5 & 10"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 2, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				tables := []int{2, 3, 4, 5, 10}
				t := tables[rng.Intn(len(tables))]
				groups := 3 + rng.Intn(7)
				ans := groups * t
				stem := LocaleText{
					EN: fmt.Sprintf("There are %d rocket pods. Each pod carries %d astronauts. How many astronauts are there altogether?", groups, t),
					ID: fmt.Sprintf("Ada %d kapsul roket. Setiap kapsul membawa %d astronot. Berapa jumlah seluruh astronot?", groups, t),
				}.For(locale)
				return buildSingleNumeric("g2_mul_tables_2_5_10", seed, 2, "num-mul-2-3-4-5-10",
					LocaleText{EN: "Multiplication Tables (2–5, 10)", ID: "Perkalian 2–5 & 10"}, "SINGAPORE_MOE", 2, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), LocaleText{EN: "astronauts", ID: "astronot"}.For(locale),
					[]string{LocaleText{EN: "Multiply the number of pods by astronauts per pod.", ID: "Kalikan jumlah kapsul dengan astronot tiap kapsul."}.For(locale)},
					nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d × %d = %d", groups, t, ans)}})
			},
		},
		{
			Code: "g2_bar_model_compare", Grade: 2, SkillCode: "wp-bar-model-compare",
			SkillName: LocaleText{EN: "Comparison Word Problems", ID: "Soal Cerita Perbandingan"},
			CurriculumCode: "HOTS_WHEEL", Difficulty: 3, HeuristicCode: "DRAW_MODELS",
			HeuristicName: LocaleText{EN: "Draw Models", ID: "Menggambar Model Batang"}, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				base := 18 + rng.Intn(30)
				more := 7 + rng.Intn(15)
				ans := base + more
				diag := &DiagramSpec{Type: "bar_model", Params: map[string]any{
					"bars": []map[string]any{
						{"label": "Rina", "units": 4, "value": fmt.Sprintf("%d", base)},
						{"label": "Budi", "units": 6, "value": fmt.Sprintf("%d + %d = ?", base, more)},
					},
				}}
				stem := LocaleText{
					EN: fmt.Sprintf("Rina has %d cosmic cards. Budi has %d more cosmic cards than Rina. How many cosmic cards does Budi have?", base, more),
					ID: fmt.Sprintf("Rina memiliki %d kartu kosmik. Budi memiliki %d kartu kosmik lebih banyak dari Rina. Berapa kartu kosmik yang dimiliki Budi?", base, more),
				}.For(locale)
				return buildSingleNumeric("g2_bar_model_compare", seed, 2, "wp-bar-model-compare",
					LocaleText{EN: "Comparison Word Problems", ID: "Soal Cerita Perbandingan"}, "HOTS_WHEEL", 3,
					"DRAW_MODELS", LocaleText{EN: "Draw Models", ID: "Menggambar Model Batang"},
					locale, stem, fmt.Sprintf("%d", ans), LocaleText{EN: "cards", ID: "kartu"}.For(locale),
					[]string{LocaleText{EN: "Open the Bar Model Builder to compare Rina's and Budi's bars.", ID: "Buka Pembuat Model Batang untuk membandingkan batang Rina dan Budi."}.For(locale)},
					diag,
					[]SolutionStep{{Text: fmt.Sprintf("%d + %d = %d", base, more, ans), Diagram: diag}})
			},
		},
		{
			Code: "g2_unit_fraction_mcq", Grade: 2, SkillCode: "frac-intro-compare",
			SkillName: LocaleText{EN: "Comparing Unit Fractions", ID: "Membandingkan Pecahan Satuan"},
			CurriculumCode: "CAMBRIDGE_0096", Difficulty: 2, InputType: InputMCQ,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				d1 := 2 + rng.Intn(3) // 2..4
				d2 := d1 + 2          // 4..6
				d3 := d2 + 2          // 6..8
				stem := LocaleText{
					EN: fmt.Sprintf("Which fraction is the GREATEST: 1/%d, 1/%d, or 1/%d?", d2, d1, d3),
					ID: fmt.Sprintf("Pecahan manakah yang PALING BESAR: 1/%d, 1/%d, atau 1/%d?", d2, d1, d3),
				}.For(locale)
				diag := &DiagramSpec{Type: "fraction_bar", Params: map[string]any{"numerator": 1, "denominator": d1}}
				return GeneratedProblem{
					Question: RenderedQuestion{
						ID: fmt.Sprintf("PL-G2-FRAC-%d", seed), TemplateCode: "g2_unit_fraction_mcq", Seed: seed,
						Grade: 2, SkillCode: "frac-intro-compare",
						SkillName: LocaleText{EN: "Comparing Unit Fractions", ID: "Membandingkan Pecahan Satuan"}.For(locale),
						CurriculumCode: "CAMBRIDGE_0096", Difficulty: 2, InputType: InputMCQ, Locale: locale,
						Stem: stem, TTSText: VerbalizeMath(stem, locale),
						Hints: []string{LocaleText{EN: "When numerators are 1, the smaller denominator gives a larger piece.", ID: "Jika pembilang sama-sama 1, penyebut lebih kecil menghasilkan potongan lebih besar."}.For(locale)},
						Options: []MCQOption{
							{Key: "A", Label: fmt.Sprintf("1/%d", d2)},
							{Key: "B", Label: fmt.Sprintf("1/%d", d1)},
							{Key: "C", Label: fmt.Sprintf("1/%d", d3)},
						},
						Diagram: diag,
					},
					Solution: WorkedSolution{
						CorrectAnswers: map[string]string{"choice": "B"},
						Steps: []SolutionStep{
							{Text: LocaleText{EN: fmt.Sprintf("1/%d is greatest because the whole is shared into the fewest equal parts.", d1), ID: fmt.Sprintf("1/%d paling besar karena 1 bagian utuh dibagi menjadi potongan paling sedikit.", d1)}.For(locale), Diagram: diag},
						},
					},
					GradingStrategy: "mcq",
				}
			},
		},

		// ==================== GRADE 3 ====================
		{
			Code: "g3_mul_6_to_9", Grade: 3, SkillCode: "num-mul-6-7-8-9",
			SkillName: LocaleText{EN: "Multiplication Tables (6–9)", ID: "Perkalian 6–9"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 2, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				a := 6 + rng.Intn(4)
				b := 4 + rng.Intn(6)
				ans := a * b
				stem := LocaleText{
					EN: fmt.Sprintf("Calculate %d × %d.", a, b),
					ID: fmt.Sprintf("Hitung hasil dari %d × %d.", a, b),
				}.For(locale)
				return buildSingleNumeric("g3_mul_6_to_9", seed, 3, "num-mul-6-7-8-9",
					LocaleText{EN: "Multiplication Tables (6–9)", ID: "Perkalian 6–9"}, "SINGAPORE_MOE", 2, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), "",
					[]string{LocaleText{EN: "Break the multiplication into friendly facts.", ID: "Pecah perkalian menjadi bilangan yang mudah dihitung."}.For(locale)},
					nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d × %d = %d", a, b, ans)}})
			},
		},
		{
			Code: "g3_div_with_remainder", Grade: 3, SkillCode: "num-div-remainder",
			SkillName: LocaleText{EN: "Division with Remainders", ID: "Pembagian dengan Sisa"},
			CurriculumCode: "KURIKULUM_MERDEKA", Difficulty: 3, InputType: InputMultiNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				divisor := 3 + rng.Intn(6)
				quotient := 5 + rng.Intn(9)
				rem := 1 + rng.Intn(divisor-1)
				dividend := divisor*quotient + rem
				stem := LocaleText{
					EN: fmt.Sprintf("%d star crystals are packed equally into boxes of %d. Find the number of full boxes and leftover crystals.", dividend, divisor),
					ID: fmt.Sprintf("%d kristal bintang dikemas sama banyak ke dalam kotak berisi %d. Tentukan jumlah kotak penuh dan sisa kristalnya.", dividend, divisor),
				}.For(locale)
				return GeneratedProblem{
					Question: RenderedQuestion{
						ID: fmt.Sprintf("PL-G3-DIV-%d", seed), TemplateCode: "g3_div_with_remainder", Seed: seed,
						Grade: 3, SkillCode: "num-div-remainder",
						SkillName: LocaleText{EN: "Division with Remainders", ID: "Pembagian dengan Sisa"}.For(locale),
						CurriculumCode: "KURIKULUM_MERDEKA", Difficulty: 3, InputType: InputMultiNumeric, Locale: locale,
						Stem: stem, TTSText: VerbalizeMath(stem, locale),
						Hints: []string{LocaleText{EN: "Divide and note the remainder.", ID: "Bagi bilangan lalu catat sisanya."}.For(locale)},
						Parts: []InputPart{
							{Key: "quotient", Label: LocaleText{EN: "Full boxes", ID: "Kotak penuh"}.For(locale)},
							{Key: "remainder", Label: LocaleText{EN: "Leftover", ID: "Sisa"}.For(locale)},
						},
					},
					Solution: WorkedSolution{
						CorrectAnswers: map[string]string{"quotient": fmt.Sprintf("%d", quotient), "remainder": fmt.Sprintf("%d", rem)},
						Steps: []SolutionStep{
							{Text: fmt.Sprintf("%d ÷ %d = %d R %d (%d × %d + %d = %d)", dividend, divisor, quotient, rem, divisor, quotient, rem, dividend)},
						},
					},
					GradingStrategy: "exact_numeric",
				}
			},
		},
		{
			Code: "g3_area_perimeter_rect", Grade: 3, SkillCode: "geo-area-perimeter-rect",
			SkillName: LocaleText{EN: "Area & Perimeter of Rectangles", ID: "Luas & Keliling Persegi Panjang"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 3, InputType: InputMultiNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				l := 6 + rng.Intn(8)
				w := 3 + rng.Intn(5)
				area := l * w
				perim := 2 * (l + w)
				diag := &DiagramSpec{Type: "rectangle_grid", Params: map[string]any{"length": l, "width": w, "unit": "cm"}}
				stem := LocaleText{
					EN: fmt.Sprintf("A solar panel is a rectangle measuring %d cm by %d cm. Find its area and perimeter.", l, w),
					ID: fmt.Sprintf("Sebuah panel surya berbentuk persegi panjang berukuran %d cm × %d cm. Hitung luas dan kelilingnya.", l, w),
				}.For(locale)
				return GeneratedProblem{
					Question: RenderedQuestion{
						ID: fmt.Sprintf("PL-G3-AP-%d", seed), TemplateCode: "g3_area_perimeter_rect", Seed: seed,
						Grade: 3, SkillCode: "geo-area-perimeter-rect",
						SkillName: LocaleText{EN: "Area & Perimeter of Rectangles", ID: "Luas & Keliling Persegi Panjang"}.For(locale),
						CurriculumCode: "SINGAPORE_MOE", Difficulty: 3, InputType: InputMultiNumeric, Locale: locale,
						Stem: stem, TTSText: VerbalizeMath(stem, locale),
						Hints: []string{LocaleText{EN: "Area = length × width; Perimeter = 2 × (length + width).", ID: "Luas = panjang × lebar; Keliling = 2 × (panjang + lebar)."}.For(locale)},
						Parts: []InputPart{
							{Key: "area", Label: LocaleText{EN: "Area", ID: "Luas"}.For(locale), Suffix: "cm²"},
							{Key: "perimeter", Label: LocaleText{EN: "Perimeter", ID: "Keliling"}.For(locale), Suffix: "cm"},
						},
						Diagram: diag,
					},
					Solution: WorkedSolution{
						CorrectAnswers: map[string]string{"area": fmt.Sprintf("%d", area), "perimeter": fmt.Sprintf("%d", perim)},
						Steps: []SolutionStep{
							{Text: fmt.Sprintf("Area = %d × %d = %d cm²", l, w, area), Diagram: diag},
							{Text: fmt.Sprintf("Perimeter = 2 × (%d + %d) = %d cm", l, w, perim)},
						},
					},
					GradingStrategy: "exact_numeric",
				}
			},
		},
		{
			Code: "g3_equiv_fraction", Grade: 3, SkillCode: "frac-add-like",
			SkillName: LocaleText{EN: "Adding Related Fractions", ID: "Penjumlahan Pecahan"},
			CurriculumCode: "CAMBRIDGE_0096", Difficulty: 3, InputType: InputFraction,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				den := 6 + rng.Intn(4) // 6..9
				n1 := 1 + rng.Intn(den/2)
				n2 := 1 + rng.Intn(den-n1)
				sumN := n1 + n2
				g := gcd(sumN, den)
				ans := fmt.Sprintf("%d/%d", sumN/g, den/g)
				stem := LocaleText{
					EN: fmt.Sprintf("Add the fractions: %d/%d + %d/%d (write as a fraction like 3/4).", n1, den, n2, den),
					ID: fmt.Sprintf("Jumlahkan pecahan berikut: %d/%d + %d/%d (tulis seperti 3/4).", n1, den, n2, den),
				}.For(locale)
				return GeneratedProblem{
					Question: RenderedQuestion{
						ID: fmt.Sprintf("PL-G3-FRAC-%d", seed), TemplateCode: "g3_equiv_fraction", Seed: seed,
						Grade: 3, SkillCode: "frac-add-like",
						SkillName: LocaleText{EN: "Adding Related Fractions", ID: "Penjumlahan Pecahan"}.For(locale),
						CurriculumCode: "CAMBRIDGE_0096", Difficulty: 3, InputType: InputFraction, Locale: locale,
						Stem: stem, TTSText: VerbalizeMath(stem, locale),
						Hints: []string{LocaleText{EN: "Add the numerators and keep the same denominator.", ID: "Jumlahkan pembilang dan pertahankan penyebut yang sama."}.For(locale)},
						Parts: []InputPart{{Key: "answer"}},
					},
					Solution: WorkedSolution{
						CorrectAnswers: map[string]string{"answer": ans},
						Steps: []SolutionStep{
							{Text: fmt.Sprintf("%d/%d + %d/%d = %d/%d = %s", n1, den, n2, den, sumN, den, ans)},
						},
					},
					GradingStrategy: "fraction_equiv",
				}
			},
		},

		// ==================== GRADE 4 ====================
		{
			Code: "g4_lcm_gcf", Grade: 4, SkillCode: "num-factors-multiples",
			SkillName: LocaleText{EN: "Factors & Multiples (GCF/LCM)", ID: "Faktor & Kelipatan (FPB/KPK)"},
			CurriculumCode: "KURIKULUM_MERDEKA", Difficulty: 3, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				pairs := [][2]int{{4, 6}, {6, 8}, {6, 9}, {8, 12}, {5, 8}, {4, 10}}
				p := pairs[rng.Intn(len(pairs))]
				lcm := (p[0] * p[1]) / gcd(p[0], p[1])
				stem := LocaleText{
					EN: fmt.Sprintf("Beacon A flashes every %d seconds and Beacon B flashes every %d seconds. If both flash together now, after how many seconds will they next flash together?", p[0], p[1]),
					ID: fmt.Sprintf("Mercusuar A menyala tiap %d detik dan Mercusuar B tiap %d detik. Jika keduanya menyala bersamaan sekarang, berapa detik lagi keduanya menyala bersamaan?", p[0], p[1]),
				}.For(locale)
				return buildSingleNumeric("g4_lcm_gcf", seed, 4, "num-factors-multiples",
					LocaleText{EN: "Factors & Multiples (GCF/LCM)", ID: "Faktor & Kelipatan (FPB/KPK)"}, "KURIKULUM_MERDEKA", 3, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", lcm), LocaleText{EN: "seconds", ID: "detik"}.For(locale),
					[]string{LocaleText{EN: "Find the Lowest Common Multiple (LCM).", ID: "Cari Kelipatan Persekutuan Terkecil (KPK)."}.For(locale)},
					nil,
					[]SolutionStep{{Text: fmt.Sprintf("LCM(%d, %d) = %d", p[0], p[1], lcm)}})
			},
		},
		{
			Code: "g4_decimal_add_sub", Grade: 4, SkillCode: "dec-add-sub",
			SkillName: LocaleText{EN: "Adding Decimals", ID: "Penjumlahan Desimal"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 3, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				aInt := 15 + rng.Intn(40) // 1.5 .. 5.4
				bInt := 12 + rng.Intn(35)
				sumInt := aInt + bInt
				aStr := fmt.Sprintf("%d.%d", aInt/10, aInt%10)
				bStr := fmt.Sprintf("%d.%d", bInt/10, bInt%10)
				ansStr := fmt.Sprintf("%d.%d", sumInt/10, sumInt%10)
				stem := LocaleText{
					EN: fmt.Sprintf("A rover drove %s km in the morning and %s km in the afternoon. What is the total distance driven?", aStr, bStr),
					ID: fmt.Sprintf("Sebuah rover melaju %d,%d km di pagi hari dan %d,%d km di sore hari. Berapa total jarak yang ditempuh?", aInt/10, aInt%10, bInt/10, bInt%10),
				}.For(locale)
				return buildSingleNumeric("g4_decimal_add_sub", seed, 4, "dec-add-sub",
					LocaleText{EN: "Adding Decimals", ID: "Penjumlahan Desimal"}, "SINGAPORE_MOE", 3, "", LocaleText{},
					locale, stem, ansStr, "km",
					[]string{LocaleText{EN: "Line up the decimal points before adding.", ID: "Sejajarkan tanda koma desimal sebelum menjumlahkan."}.For(locale)},
					nil,
					[]SolutionStep{{Text: fmt.Sprintf("%s + %s = %s km", aStr, bStr, ansStr)}})
			},
		},
		{
			Code: "g4_fraction_of_set", Grade: 4, SkillCode: "frac-of-set",
			SkillName: LocaleText{EN: "Fraction of a Set", ID: "Pecahan dari Sekumpulan Benda"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 3, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				den := 3 + rng.Intn(4) // 3..6
				num := 1 + rng.Intn(den-1)
				unitVal := 4 + rng.Intn(6)
				total := den * unitVal
				ans := num * unitVal
				stem := LocaleText{
					EN: fmt.Sprintf("There are %d moons in a star chart. %d/%d of them are icy moons. How many icy moons are there?", total, num, den),
					ID: fmt.Sprintf("Ada %d bulan di peta bintang. Sebanyak %d/%d di antaranya adalah bulan es. Berapa jumlah bulan es?", total, num, den),
				}.For(locale)
				return buildSingleNumeric("g4_fraction_of_set", seed, 4, "frac-of-set",
					LocaleText{EN: "Fraction of a Set", ID: "Pecahan dari Sekumpulan Benda"}, "SINGAPORE_MOE", 3, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), LocaleText{EN: "moons", ID: "bulan"}.For(locale),
					[]string{LocaleText{EN: fmt.Sprintf("Find 1/%d of %d first, then multiply by %d.", den, total, num), ID: fmt.Sprintf("Cari 1/%d dari %d dulu, lalu kalikan dengan %d.", den, total, num)}.For(locale)},
					nil,
					[]SolutionStep{
						{Text: fmt.Sprintf("%d ÷ %d = %d", total, den, unitVal)},
						{Text: fmt.Sprintf("%d × %d = %d", unitVal, num, ans)},
					})
			},
		},
		{
			Code: "g4_work_backwards_hots", Grade: 4, SkillCode: "hots-work-backwards",
			SkillName: LocaleText{EN: "Working Backwards", ID: "Bekerja Mundur"},
			CurriculumCode: "HOTS_WHEEL", Difficulty: 4, HeuristicCode: "WORK_BACKWARDS",
			HeuristicName: LocaleText{EN: "Work Backwards", ID: "Bekerja Mundur"}, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				start := 8 + rng.Intn(12)
				mult := 2 + rng.Intn(3)
				add := 5 + rng.Intn(10)
				finalVal := start*mult + add
				stem := LocaleText{
					EN: fmt.Sprintf("Maya thought of a secret number. She multiplied it by %d and then added %d. The result was %d. What was her secret number?", mult, add, finalVal),
					ID: fmt.Sprintf("Maya memikirkan sebuah bilangan rahasia. Ia mengalikannya dengan %d lalu menambahkan %d. Hasilnya adalah %d. Berapakah bilangan rahasia Maya?", mult, add, finalVal),
				}.For(locale)
				return buildSingleNumeric("g4_work_backwards_hots", seed, 4, "hots-work-backwards",
					LocaleText{EN: "Working Backwards", ID: "Bekerja Mundur"}, "HOTS_WHEEL", 4,
					"WORK_BACKWARDS", LocaleText{EN: "Work Backwards", ID: "Bekerja Mundur"},
					locale, stem, fmt.Sprintf("%d", start), "",
					[]string{LocaleText{EN: "Reverse the operations from the final number.", ID: "Balikkan operasi hitung dari hasil akhir."}.For(locale)},
					nil,
					[]SolutionStep{
						{Text: fmt.Sprintf("%d − %d = %d", finalVal, add, finalVal-add)},
						{Text: fmt.Sprintf("%d ÷ %d = %d", finalVal-add, mult, start)},
					})
			},
		},

		// ==================== GRADE 5 ====================
		{
			Code: "g5_percentage_discount", Grade: 5, SkillCode: "pct-discount",
			SkillName: LocaleText{EN: "Percentage & Discounts", ID: "Persentase & Diskon"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 3, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				pcts := []int{10, 20, 25, 30}
				pct := pcts[rng.Intn(len(pcts))]
				price := (4 + rng.Intn(8)) * 20 // 80..220
				discount := price * pct / 100
				ans := price - discount
				stem := LocaleText{
					EN: fmt.Sprintf("A space telescope costs $%d. During Galaxy Week it has a %d%% discount. What is the sale price?", price, pct),
					ID: fmt.Sprintf("Sebuah teleskop antariksa berharga %d koin. Saat Pekan Galaksi ada diskon %d%%. Berapa harga setelah diskon?", price, pct),
				}.For(locale)
				return buildSingleNumeric("g5_percentage_discount", seed, 5, "pct-discount",
					LocaleText{EN: "Percentage & Discounts", ID: "Persentase & Diskon"}, "SINGAPORE_MOE", 3, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", ans), "",
					[]string{LocaleText{EN: "Find the discount amount first, then subtract from the original price.", ID: "Hitung besar diskon terlebih dahulu, lalu kurangkan dari harga awal."}.For(locale)},
					nil,
					[]SolutionStep{
						{Text: fmt.Sprintf("%d%% × %d = %d", pct, price, discount)},
						{Text: fmt.Sprintf("%d − %d = %d", price, discount, ans)},
					})
			},
		},
		{
			Code: "g5_ratio_word_problem", Grade: 5, SkillCode: "ratio-two-quantities",
			SkillName: LocaleText{EN: "Ratio Word Problems", ID: "Soal Cerita Perbandingan Rasio"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 4, HeuristicCode: "DRAW_MODELS",
			HeuristicName: LocaleText{EN: "Draw Models", ID: "Menggambar Model Batang"}, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				r1 := 2 + rng.Intn(3)
				r2 := r1 + 1 + rng.Intn(3)
				unit := 4 + rng.Intn(7)
				total := (r1 + r2) * unit
				ans := r2 * unit
				stem := LocaleText{
					EN: fmt.Sprintf("The ratio of red gems to blue gems is %d : %d. There are %d gems in total. How many blue gems are there?", r1, r2, total),
					ID: fmt.Sprintf("Perbandingan permata merah dan permata biru adalah %d : %d. Jumlah seluruh permata adalah %d. Berapa banyak permata biru?", r1, r2, total),
				}.For(locale)
				return buildSingleNumeric("g5_ratio_word_problem", seed, 5, "ratio-two-quantities",
					LocaleText{EN: "Ratio Word Problems", ID: "Soal Cerita Perbandingan Rasio"}, "SINGAPORE_MOE", 4,
					"DRAW_MODELS", LocaleText{EN: "Draw Models", ID: "Menggambar Model Batang"},
					locale, stem, fmt.Sprintf("%d", ans), LocaleText{EN: "gems", ID: "permata"}.For(locale),
					[]string{LocaleText{EN: "Find the value of 1 unit by dividing total by total ratio parts.", ID: "Cari nilai 1 unit dengan membagi total dengan jumlah bagian rasio."}.For(locale)},
					nil,
					[]SolutionStep{
						{Text: fmt.Sprintf("%d + %d = %d units → 1 unit = %d ÷ %d = %d", r1, r2, r1+r2, total, r1+r2, unit)},
						{Text: fmt.Sprintf("%d × %d = %d", r2, unit, ans)},
					})
			},
		},
		{
			Code: "g5_volume_cuboid", Grade: 5, SkillCode: "geo-volume-cuboid",
			SkillName: LocaleText{EN: "Volume of Cuboids", ID: "Volume Balok"},
			CurriculumCode: "KURIKULUM_MERDEKA", Difficulty: 3, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				l := 6 + rng.Intn(6)
				w := 4 + rng.Intn(4)
				h := 3 + rng.Intn(5)
				vol := l * w * h
				stem := LocaleText{
					EN: fmt.Sprintf("A cargo container is a cuboid with length %d cm, width %d cm, and height %d cm. Find its volume.", l, w, h),
					ID: fmt.Sprintf("Sebuah peti kargo berbentuk balok dengan panjang %d cm, lebar %d cm, dan tinggi %d cm. Hitung volumenya.", l, w, h),
				}.For(locale)
				return buildSingleNumeric("g5_volume_cuboid", seed, 5, "geo-volume-cuboid",
					LocaleText{EN: "Volume of Cuboids", ID: "Volume Balok"}, "KURIKULUM_MERDEKA", 3, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", vol), "cm³",
					[]string{LocaleText{EN: "Volume = length × width × height.", ID: "Volume = panjang × lebar × tinggi."}.For(locale)},
					nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d × %d × %d = %d cm³", l, w, h, vol)}})
			},
		},
		{
			Code: "g5_average_mean", Grade: 5, SkillCode: "data-average",
			SkillName: LocaleText{EN: "Average (Mean)", ID: "Rata-rata (Mean)"},
			CurriculumCode: "CAMBRIDGE_0096", Difficulty: 3, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				avg := 14 + rng.Intn(12)
				d1 := 2 + rng.Intn(4)
				d2 := 1 + rng.Intn(3)
				a, b, c, d := avg-d1, avg+d1, avg-d2, avg+d2
				stem := LocaleText{
					EN: fmt.Sprintf("Find the average of these 4 mission scores: %d, %d, %d, and %d.", a, b, c, d),
					ID: fmt.Sprintf("Hitung rata-rata dari 4 skor misi berikut: %d, %d, %d, dan %d.", a, b, c, d),
				}.For(locale)
				return buildSingleNumeric("g5_average_mean", seed, 5, "data-average",
					LocaleText{EN: "Average (Mean)", ID: "Rata-rata (Mean)"}, "CAMBRIDGE_0096", 3, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", avg), "",
					[]string{LocaleText{EN: "Add all 4 scores and divide by 4.", ID: "Jumlahkan keempat skor lalu bagi dengan 4."}.For(locale)},
					nil,
					[]SolutionStep{{Text: fmt.Sprintf("(%d + %d + %d + %d) ÷ 4 = %d ÷ 4 = %d", a, b, c, d, 4*avg, avg)}})
			},
		},

		// ==================== GRADE 6 ====================
		{
			Code: "g6_algebra_linear", Grade: 6, SkillCode: "alg-linear-eq",
			SkillName: LocaleText{EN: "Solving Simple Algebra", ID: "Menyelesaikan Aljabar Dasar"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 4, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				x := 4 + rng.Intn(12)
				coef := 2 + rng.Intn(4)
				c := 3 + rng.Intn(11)
				rhs := coef*x + c
				stem := LocaleText{
					EN: fmt.Sprintf("Solve for x: %dx + %d = %d", coef, c, rhs),
					ID: fmt.Sprintf("Tentukan nilai x: %dx + %d = %d", coef, c, rhs),
				}.For(locale)
				return buildSingleNumeric("g6_algebra_linear", seed, 6, "alg-linear-eq",
					LocaleText{EN: "Solving Simple Algebra", ID: "Menyelesaikan Aljabar Dasar"}, "SINGAPORE_MOE", 4, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", x), "",
					[]string{LocaleText{EN: fmt.Sprintf("Subtract %d from both sides, then divide by %d.", c, coef), ID: fmt.Sprintf("Kurangi kedua ruas dengan %d, lalu bagi dengan %d.", c, coef)}.For(locale)},
					nil,
					[]SolutionStep{
						{Text: fmt.Sprintf("%dx = %d − %d = %d", coef, rhs, c, rhs-c)},
						{Text: fmt.Sprintf("x = %d ÷ %d = %d", rhs-c, coef, x)},
					})
			},
		},
		{
			Code: "g6_speed_distance_time", Grade: 6, SkillCode: "rate-speed-dst",
			SkillName: LocaleText{EN: "Speed, Distance & Time", ID: "Kecepatan, Jarak & Waktu"},
			CurriculumCode: "SINGAPORE_MOE", Difficulty: 4, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				speed := (5 + rng.Intn(7)) * 10 // 50..110
				hours := 2 + rng.Intn(4)
				dist := speed * hours
				stem := LocaleText{
					EN: fmt.Sprintf("A star shuttle travels %d km in %d hours at a constant speed. What is its speed in km/h?", dist, hours),
					ID: fmt.Sprintf("Sebuah pesawat ulang-alik menempuh jarak %d km dalam %d jam dengan kecepatan tetap. Berapa kecepatannya dalam km/jam?", dist, hours),
				}.For(locale)
				return buildSingleNumeric("g6_speed_distance_time", seed, 6, "rate-speed-dst",
					LocaleText{EN: "Speed, Distance & Time", ID: "Kecepatan, Jarak & Waktu"}, "SINGAPORE_MOE", 4, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", speed), LocaleText{EN: "km/h", ID: "km/jam"}.For(locale),
					[]string{LocaleText{EN: "Speed = Distance ÷ Time.", ID: "Kecepatan = Jarak ÷ Waktu."}.For(locale)},
					nil,
					[]SolutionStep{{Text: fmt.Sprintf("%d ÷ %d = %d", dist, hours, speed)}})
			},
		},
		{
			Code: "g6_circle_circumference", Grade: 6, SkillCode: "geo-circles",
			SkillName: LocaleText{EN: "Circumference of a Circle", ID: "Keliling Lingkaran"},
			CurriculumCode: "KURIKULUM_MERDEKA", Difficulty: 4, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				k := 1 + rng.Intn(4)
				radius := 7 * k
				circ := 2 * 22 * k // using pi = 22/7
				stem := LocaleText{
					EN: fmt.Sprintf("A circular orbit ring has a radius of %d cm. Take π = 22/7. Find its circumference.", radius),
					ID: fmt.Sprintf("Sebuah cincin orbit berbentuk lingkaran berjari-jari %d cm. Gunakan π = 22/7. Hitung kelilingnya.", radius),
				}.For(locale)
				return buildSingleNumeric("g6_circle_circumference", seed, 6, "geo-circles",
					LocaleText{EN: "Circumference of a Circle", ID: "Keliling Lingkaran"}, "KURIKULUM_MERDEKA", 4, "", LocaleText{},
					locale, stem, fmt.Sprintf("%d", circ), "cm",
					[]string{LocaleText{EN: "Circumference = 2 × π × radius.", ID: "Keliling = 2 × π × jari-jari."}.For(locale)},
					nil,
					[]SolutionStep{{Text: fmt.Sprintf("2 × (22/7) × %d = %d cm", radius, circ)}})
			},
		},
		{
			Code: "g6_supposition_hots", Grade: 6, SkillCode: "hots-make-supposition",
			SkillName: LocaleText{EN: "Make Supposition (Chickens & Rabbits)", ID: "Membuat Pengandaian"},
			CurriculumCode: "CHALLENGING_MATHS", Difficulty: 5, HeuristicCode: "MAKE_SUPPOSITION",
			HeuristicName: LocaleText{EN: "Make Supposition", ID: "Membuat Pengandaian"}, InputType: InputSingleNumeric,
			Generate: func(rng *rand.Rand, locale string, seed int64) GeneratedProblem {
				twoWheel := 4 + rng.Intn(6)
				fourWheel := 3 + rng.Intn(6)
				totalVehicles := twoWheel + fourWheel
				totalWheels := 2*twoWheel + 4*fourWheel
				stem := LocaleText{
					EN: fmt.Sprintf("In a lunar garage there are %d vehicles made up of 2-wheel moon bikes and 4-wheel moon rovers. There are %d wheels altogether. How many 4-wheel moon rovers are there?", totalVehicles, totalWheels),
					ID: fmt.Sprintf("Di garasi bulan terdapat %d kendaraan yang terdiri dari motor bulan roda 2 dan rover bulan roda 4. Total ada %d roda. Berapa banyak rover bulan roda 4?", totalVehicles, totalWheels),
				}.For(locale)
				return buildSingleNumeric("g6_supposition_hots", seed, 6, "hots-make-supposition",
					LocaleText{EN: "Make Supposition (Chickens & Rabbits)", ID: "Membuat Pengandaian"}, "CHALLENGING_MATHS", 5,
					"MAKE_SUPPOSITION", LocaleText{EN: "Make Supposition", ID: "Membuat Pengandaian"},
					locale, stem, fmt.Sprintf("%d", fourWheel), LocaleText{EN: "rovers", ID: "rover"}.For(locale),
					[]string{LocaleText{EN: "Assume all vehicles are 2-wheel bikes first, then divide the extra wheels by 2.", ID: "Andaikan semua kendaraan beroda 2 dulu, lalu bagi selisih rodanya dengan 2."}.For(locale)},
					nil,
					[]SolutionStep{
						{Text: fmt.Sprintf("If all %d were 2-wheel: %d × 2 = %d wheels", totalVehicles, totalVehicles, 2*totalVehicles)},
						{Text: fmt.Sprintf("Extra wheels: %d − %d = %d → %d ÷ 2 = %d rovers", totalWheels, 2*totalVehicles, totalWheels-2*totalVehicles, totalWheels-2*totalVehicles, fourWheel)},
					})
			},
		},
	}
	return append(base, extraTemplates()...)
}

func buildSingleNumeric(
	code string,
	seed int64,
	grade int,
	skillCode string,
	skillName LocaleText,
	curriculumCode string,
	difficulty int,
	heuristicCode string,
	heuristicName LocaleText,
	locale string,
	stem string,
	answer string,
	suffix string,
	hints []string,
	diagram *DiagramSpec,
	steps []SolutionStep,
) GeneratedProblem {
	return GeneratedProblem{
		Question: RenderedQuestion{
			ID:             fmt.Sprintf("PL-G%d-%s-%d", grade, code, seed),
			TemplateCode:   code,
			Seed:           seed,
			Grade:          grade,
			SkillCode:      skillCode,
			SkillName:      skillName.For(locale),
			CurriculumCode: curriculumCode,
			Difficulty:     difficulty,
			HeuristicCode:  heuristicCode,
			HeuristicName:  heuristicName.For(locale),
			InputType:      InputSingleNumeric,
			Locale:         locale,
			Stem:           stem,
			TTSText:        VerbalizeMath(stem, locale),
			Hints:          hints,
			Parts:          []InputPart{{Key: "answer", Suffix: suffix}},
			Diagram:        diagram,
		},
		Solution: WorkedSolution{
			CorrectAnswers: map[string]string{"answer": answer},
			Steps:          steps,
		},
		GradingStrategy: "exact_numeric",
	}
}

// GenerateByCode deterministically constructs a problem from (templateCode, seed, locale).
func GenerateByCode(code string, seed int64, locale string) (GeneratedProblem, bool) {
	if locale != "id" {
		locale = "en"
	}
	for _, tmpl := range Registry() {
		if tmpl.Code == code {
			rng := rand.New(rand.NewSource(seed))
			return tmpl.Generate(rng, locale, seed), true
		}
	}
	return GeneratedProblem{}, false
}

// TemplatesForGrade returns templates matching the given elementary grade (1..6).
func TemplatesForGrade(grade int) []TemplateMeta {
	all := Registry()
	out := make([]TemplateMeta, 0)
	for _, t := range all {
		if t.Grade == grade {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return all
	}
	return out
}
